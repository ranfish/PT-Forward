/*
 * vf_dovi_reshape.c — CPU Dolby Vision BL reshaping (PT-Forward custom)
 *
 * Applies the DoVi RPU reshaping curves + color matrices to a Dolby Vision
 * base layer on the CPU, producing a plain HDR10 (PQ/bt.2020) frame that the
 * zimg software conversion chain (vo=image) can process like any HDR10 disc.
 *
 * This replicates the math of libplacebo's pl_shader_dovi_reshape +
 * pl_shader_decode_color_ex (DOLBYVISION branch), see:
 *   libplacebo src/shaders/colorspace.c  (curves, PQ/LMS stages)
 *   libplacebo src/colorspace.c          (pl_color_repr_decode offset fold)
 *
 * The metadata (struct pl_dovi_metadata) is attached to mp_image params by
 * mp_image_from_av_frame() whenever the decoder exported
 * AV_FRAME_DATA_DOVI_METADATA side data (FFmpeg >= 7 with dovi_rpu support).
 *
 * EL residual (FEL profile 7) is not composed; the reshaped BL alone yields a
 * viewable HDR10-grade image (MEL equivalent). Non-DoVi frames pass through
 * untouched, so this filter is safe to enable unconditionally.
 */

#include <math.h>
#include <string.h>

#include <libavcodec/avcodec.h>
#include <libavformat/avformat.h>
#include <libavutil/buffer.h>
#include <libavutil/common.h>
#include <libavutil/dovi_meta.h>
#include <libavutil/frame.h>
#include <libavutil/opt.h>

#include <libplacebo/colorspace.h>

#include "common/common.h"
#include "common/msg.h"

#include "filters/filter.h"
#include "filters/filter_internal.h"
#include "filters/user_filters.h"

#include "video/img_format.h"
#include "video/mp_image.h"

#include "options/m_option.h"

// ---------------------------------------------------------------------------
// PQ (SMPTE ST 2084) constants
// ---------------------------------------------------------------------------

#define PQ_M1 (2610.0 / 16384.0)
#define PQ_M2 (2523.0 / 4096.0 * 128.0)
#define PQ_C1 (3424.0 / 4096.0)
#define PQ_C2 (2413.0 / 4096.0 * 32.0)
#define PQ_C3 (2392.0 / 4096.0 * 32.0)

// BT.2020-referred HPE LMS -> RGB (hard-coded in libplacebo dovi branch)
static const float DOVI_LMS2RGB[3][3] = {
    { 3.06441879f, -2.16597676f,  0.10155818f},
    {-0.65612108f,  1.78554118f, -0.12943749f},
    { 0.01736321f, -0.04725154f,  1.03004253f},
};

// §59.300 附五：vf 直出 SDR——PQ 线性光经 mobius tone map（与引擎原 lavfi
// tonemap=mobius npl=100 同款观感）后 bt709 编码，摆脱 mpv zimg 对 PQ 无 tone map
// 的硬裁剪（亮场景白花根因）。
#define BT709_KR 0.2126f
#define BT709_KG 0.7152f
#define BT709_KB 0.0722f

#define PQ_LUT_N 8192

struct dovi_curve {
    int num_pivots;          // [2, 9]
    float pivots[9];         // normalized [0, 1]
    int method[8];           // 0 = polynomial, 1 = MMR
    float poly[8][3];        // x^0, x^1, x^2
    int mmr_order[8];        // [1, 3]
    float mmr_cst[8];
    float mmr[8][3][7];
};

// Ring of RPU metadata extracted from an enhancement-layer stream.
#define EL_RING_N 32

struct el_meta_entry {
    double pts;                       // seconds
    struct pl_dovi_metadata meta;
    float tm_peak;                    // mobius peak = source_max_nits / 100
};

struct el_state {
    AVFormatContext *fmt;
    AVCodecContext *dec;
    int stream_idx;
    bool eof;
    double last_pts;                  // last decoded EL frame pts (secs)
    struct el_meta_entry ring[EL_RING_N];
    int ring_len;
};

struct dovi_opts {
    char *el_source;
    double until;      // 只 reshape pts >= until 的帧（前导帧直通省时；<0 = 全部）
};

struct priv {
    struct mp_log *log;
    struct dovi_opts *opts;

    // Optional enhancement-layer RPU source
    struct el_state el;
    bool el_failed;

    // Prepared per-metadata state
    const void *meta_key; // identity of prepared metadata
    bool prepared;
    bool fmt_warned;

    float sig_scale;         // raw sample -> normalized signal
    float mat_nonlin[3][3];  // ycc -> signal matrix (dovi->nonlinear)
    float mat_nonlin_c[3];   // folded: -mat * offset * levels_scale
    float mat_out[3][3];     // LMS2RGB * dovi->linear
    struct dovi_curve curves[3];
    float cur_peak;          // 当前 tonemap LUT 的 peak（0=未建）

    // PQ transfer LUTs (built once)
    float eotf_lut[PQ_LUT_N + 1]; // PQ signal -> linear
    float oetf_lut[PQ_LUT_N + 1]; // linear -> PQ signal
    float tm_lut[PQ_LUT_N + 1];   // linear(0..1=10000nit) -> tonemapped(0..1)
    float g709_lut[PQ_LUT_N + 1]; // tonemapped linear -> bt709 signal
};

// ---------------------------------------------------------------------------

static float pq_eotf(float x)
{
    // §59.300 附十二：lms 可 >1（nonlin 矩阵系数至 2.15，亮+色度像素越界）——
    // 公式对 x≤~1.9 单调可导；曾 lut_lookup 硬夹 [0,1] 致三通道夹断点随色度漂移
    // → 亮区彩色斑块=花屏（夜戏永不越界所以干净）
    if (x > 1.0f)
        x = MPCLAMP(x, 0.0f, 1.85f);
    float v = powf(x, 1.0f / PQ_M2);
    v = fmaxf(v - PQ_C1, 0.0f) / (PQ_C2 - PQ_C3 * v);
    return powf(v, 1.0f / PQ_M1);
}

static float pq_eotff(float x)
{
    return pq_eotf(x);
}

static float pq_oetf(float x)
{
    float v = powf(x, PQ_M1);
    v = (PQ_C1 + PQ_C2 * v) / (1.0f + PQ_C3 * v);
    return powf(v, PQ_M2);
}

static void build_pq_luts(struct priv *p)
{
    for (int i = 0; i <= PQ_LUT_N; i++) {
        float x = (float)i / PQ_LUT_N;
        p->eotf_lut[i] = pq_eotf(x);
        p->oetf_lut[i] = pq_oetf(x);
    }
}

// §59.300 附十一：SDR 峰归一化（npl=100 语义：×100 = nit/100）+ BT.2390 hermite
// 软肩——全部用户验收点交叉验证统一（1nit→11 ✓ 2nit→23 ✓ 白天 10nit→74=PotPlayer
// 较淡 ✓ 高光滚降 ✓）。此前 ×100"全白"=空 until 直通帧假象
#define TM_GAIN 100.0f
#define TM_KNEE 0.75f
#define TM_SHOULDER 3.0f

static void build_tonemap_luts(struct priv *p, float peak)
{
    (void)peak;
    for (int i = 0; i <= PQ_LUT_N; i++) {
        float u = (float)i / PQ_LUT_N;       // 输入索引 = 线性 0..1（10000nit）
        float x = u * TM_GAIN;               // nit/100（SDR 峰相对域）
        float tm;
        if (x <= TM_KNEE) {
            tm = x;
        } else {
            float t = MPCLAMP((x - TM_KNEE) / (TM_SHOULDER - TM_KNEE), 0.0f, 1.0f);
            float t2 = t * t, t3 = t2 * t;
            tm = (2*t3 - 3*t2 + 1) * TM_KNEE + (t3 - 2*t2 + t) * (1.0f - TM_KNEE);
        }
        p->tm_lut[i] = MPCLAMP(tm, 0.0f, 1.0f);
        // g709 = 纯 bt709 OETF（索引 = 已 tonemap 的 SDR 相对线性）
        float v = u >= 0.018f ? 1.099f * powf(u, 0.45f) - 0.099f : 4.5f * u;
        p->g709_lut[i] = MPCLAMP(v, 0.0f, 1.0f);
    }
}

static float lut_lookup(const float *lut, float x)
{
    if (x <= 0.0f)
        return lut[0];
    if (x >= 1.0f)
        return lut[PQ_LUT_N];
    float f = x * PQ_LUT_N;
    int i = (int)f;
    float t = f - i;
    return lut[i] + (lut[i + 1] - lut[i]) * t;
}

// ---------------------------------------------------------------------------

static void prepare_curves(const struct pl_dovi_metadata *dovi,
                           struct dovi_curve curves[3])
{
    for (int c = 0; c < 3; c++) {
        const struct pl_reshape_data *src = &dovi->comp[c];
        struct dovi_curve *dst = &curves[c];
        dst->num_pivots = src->num_pivots;
        for (int i = 0; i < src->num_pivots && i < 9; i++)
            dst->pivots[i] = src->pivots[i];
        for (int i = 0; i < src->num_pivots - 1 && i < 8; i++) {
            dst->method[i] = src->method[i];
            for (int k = 0; k < 3; k++)
                dst->poly[i][k] = src->poly_coeffs[i][k];
            dst->mmr_order[i] = src->mmr_order[i];
            dst->mmr_cst[i] = src->mmr_constant[i];
            for (int j = 0; j < 3; j++)
                for (int k = 0; k < 7; k++)
                    dst->mmr[i][j][k] = src->mmr_coeffs[i][j][k];
        }
    }
}

// Evaluate the reshaping curve for one component.
// sig: clamped [0,1] signal triple; returns pivot-clamped output.
static inline float eval_curve(const struct dovi_curve *cv, int c,
                               const float sig[3])
{
    if (cv->num_pivots < 2)
        return sig[c];

    float s = sig[c];

    // Select piece: idx = max{ i in [0, num_pivots-2] : s >= pivots[i] }
    int idx = 0;
    for (int i = 1; i < cv->num_pivots - 1; i++) {
        if (s >= cv->pivots[i])
            idx = i;
    }

    float out;
    if (cv->method[idx] == 0) { // polynomial
        const float *pc = cv->poly[idx];
        out = (pc[2] * s + pc[1]) * s + pc[0];
    } else {                    // MMR
        int order = MPMIN(MPMAX(cv->mmr_order[idx], 1), 3);
        const float (*mc)[7] = cv->mmr[idx];
        const float s0 = sig[0], s1 = sig[1], s2 = sig[2];
        const float x0 = s0 * s1, x1 = s0 * s2, x2 = s1 * s2, x3 = x0 * s2;
        out = cv->mmr_cst[idx];
        // order 1
        out += mc[0][0] * s0 + mc[0][1] * s1 + mc[0][2] * s2 +
               mc[0][3] * x0 + mc[0][4] * x1 + mc[0][5] * x2 + mc[0][6] * x3;
        if (order >= 2) {
            const float q0 = s0 * s0, q1 = s1 * s1, q2 = s2 * s2;
            const float y0 = x0 * x0, y1 = x1 * x1, y2 = x2 * x2, y3 = x3 * x3;
            out += mc[1][0] * q0 + mc[1][1] * q1 + mc[1][2] * q2 +
                   mc[1][3] * y0 + mc[1][4] * y1 + mc[1][5] * y2 + mc[1][6] * y3;
        }
        if (order >= 3) {
            const float q0 = s0 * s0 * s0, q1 = s1 * s1 * s1, q2 = s2 * s2 * s2;
            const float y0 = x0 * x0 * x0, y1 = x1 * x1 * x1,
                        y2 = x2 * x2 * x2, y3 = x3 * x3 * x3;
            out += mc[2][0] * q0 + mc[2][1] * q1 + mc[2][2] * q2 +
                   mc[2][3] * y0 + mc[2][4] * y1 + mc[2][5] * y2 + mc[2][6] * y3;
        }
    }

    return MPCLAMP(out, cv->pivots[0], cv->pivots[cv->num_pivots - 1]);
}

// Full per-pixel pipeline: normalized YCbCr signal -> PQ RGB [0, 1]
static inline void reshape_pixel(struct priv *p, const float in[3], float out[3])
{
    float sig[3] = { MPCLAMP(in[0], 0.0f, 1.0f),
                     MPCLAMP(in[1], 0.0f, 1.0f),
                     MPCLAMP(in[2], 0.0f, 1.0f) };

    float vdr[3];
    for (int c = 0; c < 3; c++)
        vdr[c] = eval_curve(&p->curves[c], c, sig);

    // ycc -> "PQ LMS": mat * vdr + c   (c = -mat * offset * levels_scale)
    float lms[3];
    for (int i = 0; i < 3; i++)
        lms[i] = p->mat_nonlin[i][0] * vdr[0] + p->mat_nonlin[i][1] * vdr[1] +
                 p->mat_nonlin[i][2] * vdr[2] + p->mat_nonlin_c[i];

    // PQ EOTF -> linear LMS -> matrix -> linear RGB
    // lms>1（亮+色度越界）走直算外推，保留通道间相对关系（附十二）
    float lin[3];
    for (int i = 0; i < 3; i++) {
        if (lms[i] > 1.0f) {
            lin[i] = pq_eotf(MPCLAMP(lms[i], 0.0f, 1.85f));
        } else if (lms[i] < 0.0f) {
            lin[i] = 0.0f;
        } else {
            lin[i] = lut_lookup(p->eotf_lut, lms[i]);
        }
    }

    float rgb[3];
    for (int i = 0; i < 3; i++)
        rgb[i] = p->mat_out[i][0] * lin[0] + p->mat_out[i][1] * lin[1] +
                 p->mat_out[i][2] * lin[2];

    // §59.300 附十七：纯 PQ OETF 直出（P5 正确形态=libplacebo/mpv 同构：reshape→PQ，
    // 显示层转换交 lavfi——附十一内部 tonemap+附十三 chroma_sat 与 lavfi 叠加=
    // 双重色调映射+过饱和，G 通道压 0 实证）
    for (int i = 0; i < 3; i++)
        out[i] = lut_lookup(p->oetf_lut, MPCLAMP(rgb[i], 0.0f, 1.0f));
    }

// ---------------------------------------------------------------------------

static bool prepare(struct priv *p, const struct pl_dovi_metadata *dovi,
                    const struct pl_bit_encoding *frame_bits, float tm_peak)
{
    if (!dovi)
        return false;

    if (p->prepared && p->meta_key == dovi && p->cur_peak == tm_peak)
        return true;
    p->cur_peak = tm_peak;
    build_tonemap_luts(p, tm_peak);

    // Signal normalization, matching pl_color_repr_normalize (limited) with
    // the texture sample divisor: sample * 2^(tex-col) / (2^tex - 1).
    int tex_bits = frame_bits->sample_depth ? frame_bits->sample_depth : 16;
    int col_bits = frame_bits->color_depth ? frame_bits->color_depth : tex_bits;
    if (tex_bits < col_bits || tex_bits > 16)
        return false;
    p->sig_scale = (float)(1LL << (tex_bits - col_bits)) /
                   (float)((1LL << tex_bits) - 1);

    // levels scale from pl_color_repr_decode: (2^tex)/((2^tex)-1)
    float levels_scale = (float)(1LL << tex_bits) / (float)((1LL << tex_bits) - 1);

    for (int i = 0; i < 3; i++) {
        for (int j = 0; j < 3; j++)
            p->mat_nonlin[i][j] = dovi->nonlinear.m[i][j];
        p->mat_nonlin_c[i] = -(dovi->nonlinear.m[i][0] * dovi->nonlinear_offset[0] +
                               dovi->nonlinear.m[i][1] * dovi->nonlinear_offset[1] +
                               dovi->nonlinear.m[i][2] * dovi->nonlinear_offset[2])
                             * levels_scale;
    }

    // mat_out = LMS2RGB * dovi->linear (same composition as
    // pl_matrix3x3_mul(&dovi_lms2rgb, &repr->dovi->linear) in libplacebo)
    const pl_matrix3x3 lin = dovi->linear;
    for (int i = 0; i < 3; i++) {
        for (int j = 0; j < 3; j++) {
            p->mat_out[i][j] = DOVI_LMS2RGB[i][0] * lin.m[0][j] +
                               DOVI_LMS2RGB[i][1] * lin.m[1][j] +
                               DOVI_LMS2RGB[i][2] * lin.m[2][j];
        }
    }

    prepare_curves(dovi, p->curves);

    p->meta_key = dovi;
    p->prepared = true;
    MP_INFO(p, "DoVi reshape prepared (comp0 pivots: %d)\n",
            p->curves[0].num_pivots);
    MP_VERBOSE(p, "DoVi meta dump: sig_scale=%.6f\n"
               "  nonlin=[%.4f %.4f %.4f; %.4f %.4f %.4f; %.4f %.4f %.4f] "
               "c=[%.4f %.4f %.4f]\n"
               "  lin=[%.4f %.4f %.4f; %.4f %.4f %.4f; %.4f %.4f %.4f]\n"
               "  comp0: pivots[0]=%.4f pivots[n-1]=%.4f method0=%d "
               "poly0=[%g %g %g] mmr0ord=%d\n",
               p->sig_scale,
               p->mat_nonlin[0][0], p->mat_nonlin[0][1], p->mat_nonlin[0][2],
               p->mat_nonlin[1][0], p->mat_nonlin[1][1], p->mat_nonlin[1][2],
               p->mat_nonlin[2][0], p->mat_nonlin[2][1], p->mat_nonlin[2][2],
               p->mat_nonlin_c[0], p->mat_nonlin_c[1], p->mat_nonlin_c[2],
               p->mat_out[0][0], p->mat_out[0][1], p->mat_out[0][2],
               p->mat_out[1][0], p->mat_out[1][1], p->mat_out[1][2],
               p->mat_out[2][0], p->mat_out[2][1], p->mat_out[2][2],
               p->curves[0].pivots[0],
               p->curves[0].pivots[p->curves[0].num_pivots - 1],
               p->curves[0].method[0],
               p->curves[0].poly[0][0], p->curves[0].poly[0][1],
               p->curves[0].poly[0][2], p->curves[0].mmr_order[0]);
    // Sample pixel pipeline trace: mid-grey limited YCbCr
    {
        float in[3] = { (520.0f / 1023.0f), (512.0f / 1023.0f), (512.0f / 1023.0f) };
        float out[3];
        reshape_pixel(p, in, out);
        MP_VERBOSE(p, "DoVi sample: in=[%.4f %.4f %.4f] -> out=[%.4f %.4f %.4f]\n",
                   in[0], in[1], in[2], out[0], out[1], out[2]);
    }
    return true;
}

// Precompute bilinear chroma tap weights. Chroma samples are assumed centered
// at (2i + 0.5, 2j + 0.5) in the luma grid (MPEG-1 style siting).
static void build_chroma_taps(void *ta, int luma_dim, int chroma_dim,
                              int **out_i0, float **out_t)
{
    int *i0 = talloc_array(ta, int, luma_dim);
    float *t = talloc_array(ta, float, luma_dim);
    for (int n = 0; n < luma_dim; n++) {
        float u = ((float)n - 0.5f) * 0.5f;
        int a = (int)floorf(u);
        float ft = u - a;
        if (chroma_dim < 2) {
            a = 0;
            ft = 0.0f;
        } else {
            if (a < 0) {
                a = 0;
                ft = 0.0f;
            }
            if (a > chroma_dim - 2) {
                a = chroma_dim - 2;
                ft = 1.0f;
            }
        }
        i0[n] = a;
        t[n] = ft;
    }
    *out_i0 = i0;
    *out_t = t;
}

static bool reshape_frame(struct priv *p, struct mp_image *mpi,
                          struct mp_image *dmpi)
{
    const int w = mpi->w, h = mpi->h;
    const int cw = (w + 1) / 2, ch = (h + 1) / 2;

    const uint16_t *yp = (const uint16_t *)mpi->planes[0];
    const uint16_t *cbp = (const uint16_t *)mpi->planes[1];
    const uint16_t *crp = (const uint16_t *)mpi->planes[2];
    const int ys = mpi->stride[0] / 2, cs = mpi->stride[1] / 2;

    uint16_t *dy = (uint16_t *)dmpi->planes[0];
    uint16_t *dcb = (uint16_t *)dmpi->planes[1];
    uint16_t *dcr = (uint16_t *)dmpi->planes[2];
    const int dys = dmpi->stride[0] / 2, dcs = dmpi->stride[1] / 2;

    const float ss = p->sig_scale;
    const float cb_gain = 0.5f / (1.0f - 0.0593f);
    const float cr_gain = 0.5f / (1.0f - 0.2627f);


    void *ta = talloc_new(p);
    int *xi0, *yj0;
    float *xt, *yt;
    build_chroma_taps(ta, w, cw, &xi0, &xt);
    build_chroma_taps(ta, h, ch, &yj0, &yt);

    // Accumulators for output chroma (2x2 block average), normalized [0,1]
    float *acc = talloc_zero_array(ta, float, cw * ch * 2);
    if (!acc) {
        talloc_free(ta);
        return false;
    }

    for (int y = 0; y < h; y++) {
        const int yrow = y * ys;
        const int arow = (y / 2) * cw * 2;
        const int j0 = yj0[y];
        const int j1 = MPMIN(j0 + 1, ch - 1);
        const float ty = yt[y];
        const uint16_t *cb_r0 = cbp + j0 * cs;
        const uint16_t *cb_r1 = cbp + j1 * cs;
        const uint16_t *cr_r0 = crp + j0 * cs;
        const uint16_t *cr_r1 = crp + j1 * cs;
        const float wy0 = 1.0f - ty, wy1 = ty;
        for (int x = 0; x < w; x++) {
            const int i0 = xi0[x];
            const int i1 = MPMIN(i0 + 1, cw - 1);
            const float tx = xt[x];
            const float wx0 = 1.0f - tx, wx1 = tx;
            float cbv = (cb_r0[i0] * wx0 + cb_r0[i1] * wx1) * wy0 +
                        (cb_r1[i0] * wx0 + cb_r1[i1] * wx1) * wy1;
            float crv = (cr_r0[i0] * wx0 + cr_r0[i1] * wx1) * wy0 +
                        (cr_r1[i0] * wx0 + cr_r1[i1] * wx1) * wy1;
            float in[3] = { yp[yrow + x] * ss, cbv * ss, crv * ss };

            float o[3];
            reshape_pixel(p, in, o);

            // PQ RGB -> BT.2020 limited YCbCr (10-bit)
            float ylin = 0.2627f * o[0] + 0.6780f * o[1] + 0.0593f * o[2];
            float cb = (o[2] - ylin) * cb_gain + 0.5f;
            float cr = (o[0] - ylin) * cr_gain + 0.5f;

            dy[y * dys + x] = MPCLAMP(lrintf(64.0f + ylin * 876.0f), 0, 1023);

            float *a = &acc[arow + (x / 2) * 2];
            a[0] += cb;
            a[1] += cr;
        }
    }

    // Average the 2x2 block contributions and quantize chroma.
    // cb/cr 为 0.5 中心值：量化 = 512 + (v-0.5)*448（§59.300 附四：双重中心偏置
    // 会全帧 +224 → 品红屏）
    for (int cy = 0; cy < ch; cy++) {
        int ry = MPMIN(2, h - cy * 2);
        for (int cx = 0; cx < cw; cx++) {
            int rx = MPMIN(2, w - cx * 2);
            int n = ry * rx;
            const float *a = &acc[(cy * cw + cx) * 2];
            float cb = a[0] / n, cr = a[1] / n;
            dcb[cy * dcs + cx] = MPCLAMP(lrintf(512.0f + (cb - 0.5f) * 896.0f), 0, 1023);
            dcr[cy * dcs + cx] = MPCLAMP(lrintf(512.0f + (cr - 0.5f) * 896.0f), 0, 1023);
        }
    }

    talloc_free(ta);
    return true;
}

// ---------------------------------------------------------------------------
// Enhancement-layer RPU extraction
//
// Profile 7 m2ts discs carry the RPU in the 1080p enhancement-layer video
// stream, not in the 4K base layer. We decode the EL stream through lavc
// (whose hevcdec parses NAL62 RPUs and attaches AV_FRAME_DATA_DOVI_METADATA
// side data) and keep a small PTS-indexed ring of mapped metadata. Pixel
// accuracy of the EL decode is irrelevant; only the side data matters.
// ---------------------------------------------------------------------------

static void el_close(struct el_state *el)
{
    if (el->dec) {
        avcodec_free_context(&el->dec);
        el->dec = NULL;
    }
    if (el->fmt) {
        avformat_close_input(&el->fmt);
        el->fmt = NULL;
    }
    el->ring_len = 0;
    el->last_pts = MP_NOPTS_VALUE;
    el->eof = false;
}

static bool el_open(struct priv *p, const char *path)
{
    struct el_state *el = &p->el;
    el_close(el);

    if (avformat_open_input(&el->fmt, path, NULL, NULL) < 0) {
        MP_ERR(p, "dovi_reshape: cannot open EL source '%s'\n", path);
        return false;
    }
    if (avformat_find_stream_info(el->fmt, NULL) < 0) {
        MP_ERR(p, "dovi_reshape: no stream info in EL source\n");
        goto fail;
    }

    // Pick the smallest-resolution HEVC video stream (the EL).
    int best = -1;
    int best_h = INT_MAX;
    for (unsigned i = 0; i < el->fmt->nb_streams; i++) {
        AVStream *st = el->fmt->streams[i];
        if (st->codecpar->codec_type != AVMEDIA_TYPE_VIDEO ||
            st->codecpar->codec_id != AV_CODEC_ID_HEVC)
            continue;
        if (st->codecpar->height < best_h) {
            best_h = st->codecpar->height;
            best = i;
        }
    }
    if (best < 0) {
        MP_ERR(p, "dovi_reshape: EL source has no HEVC video stream\n");
        goto fail;
    }
    el->stream_idx = best;

    const AVCodec *codec = avcodec_find_decoder(AV_CODEC_ID_HEVC);
    el->dec = avcodec_alloc_context3(codec);
    if (!el->dec)
        goto fail;
    if (avcodec_parameters_to_context(el->dec, el->fmt->streams[best]->codecpar) < 0)
        goto fail;
    // Pixel data is discarded; keep it cheap.
    el->dec->thread_count = 1;
    if (avcodec_open2(el->dec, codec, NULL) < 0) {
        MP_ERR(p, "dovi_reshape: cannot open HEVC decoder for EL\n");
        goto fail;
    }

    MP_INFO(p, "dovi_reshape: EL source '%s' stream %d (%dx%d)\n",
            path, best, el->fmt->streams[best]->codecpar->width, best_h);
    return true;

fail:
    el_close(el);
    return false;
}

static void el_push_meta(struct el_state *el, double pts,
                         const AVDOVIMetadata *meta)
{
    struct el_meta_entry *e;
    if (el->ring_len < EL_RING_N) {
        e = &el->ring[el->ring_len++];
    } else {
        memmove(&el->ring[0], &el->ring[1], sizeof(el->ring[0]) * (EL_RING_N - 1));
        e = &el->ring[EL_RING_N - 1];
    }
    e->pts = pts;

    // L1 元数据：source_max_pq → tone map peak（nits/100；缺省 1000nit=10）
    e->tm_peak = 10.0f;
    {
        // AVDOVIColorMetadata 的 L1 藏在 color 里（dovi_meta.h source_min/max_pq）
        const AVDOVIColorMetadata *cm = av_dovi_get_color(meta);
        if (cm->source_max_pq > 0) {
            float max_nits = pq_eotff((float)cm->source_max_pq / 4095.0f) * 10000.0f;
            if (max_nits >= 100.0f)
                e->tm_peak = max_nits / 100.0f;
        }
    }

    // Own AVDOVIMetadata -> pl_dovi_metadata mapping. libplacebo <v7.364
    // refuses FEL (disable_residual_flag=0) metadata inside
    // pl_map_avdovi_metadata and silently leaves the output zeroed; Profile 7
    // FEL reshaping without the EL residual is still valid, so map here.
    const AVDOVIRpuDataHeader *header = av_dovi_get_header(meta);
    const AVDOVIDataMapping *mapping = av_dovi_get_mapping(meta);
    const AVDOVIColorMetadata *color = av_dovi_get_color(meta);
    struct pl_dovi_metadata *out = &e->meta;
    memset(out, 0, sizeof(*out));

    for (int i = 0; i < 3; i++)
        out->nonlinear_offset[i] = av_q2d(color->ycc_to_rgb_offset[i]);
    for (int i = 0; i < 9; i++) {
        float *nonlinear = &out->nonlinear.m[0][0];
        float *linear = &out->linear.m[0][0];
        nonlinear[i] = av_q2d(color->ycc_to_rgb_matrix[i]);
        linear[i] = av_q2d(color->rgb_to_lms_matrix[i]);
    }

    float pivot_scale = 1.0f / (float)((1 << header->bl_bit_depth) - 1);
    float coef_scale = 1.0f / (float)(1LL << header->coef_log2_denom);
    for (int c = 0; c < 3; c++) {
        const AVDOVIReshapingCurve *src = &mapping->curves[c];
        struct pl_reshape_data *dst = &out->comp[c];
        dst->num_pivots = src->num_pivots;
        for (int i = 0; i < src->num_pivots && i < 9; i++)
            dst->pivots[i] = pivot_scale * src->pivots[i];
        for (int i = 0; i < src->num_pivots - 1 && i < 8; i++) {
            dst->method[i] = src->mapping_idc[i];
            switch (src->mapping_idc[i]) {
            case AV_DOVI_MAPPING_POLYNOMIAL:
                for (int k = 0; k < 3; k++)
                    dst->poly_coeffs[i][k] = coef_scale * src->poly_coef[i][k];
                break;
            case AV_DOVI_MAPPING_MMR:
                dst->mmr_order[i] = src->mmr_order[i];
                dst->mmr_constant[i] = coef_scale * src->mmr_constant[i];
                for (int j = 0; j < 3; j++)
                    for (int k = 0; k < 7; k++)
                        dst->mmr_coeffs[i][j][k] = coef_scale * src->mmr_coef[i][j][k];
                break;
            }
        }
    }
}

static double frame_pts_secs(AVCodecContext *dec, AVFrame *frame,
                             AVStream *st)
{
    int64_t pts = frame->pts != AV_NOPTS_VALUE ? frame->pts : frame->pkt_dts;
    if (pts == AV_NOPTS_VALUE)
        return MP_NOPTS_VALUE;
    return (double)pts * st->time_base.num / st->time_base.den;
}

// Decode EL packets until we have metadata covering target pts.
static const struct el_meta_entry *el_lookup_entry(struct priv *p, double target)
{
    struct el_state *el = &p->el;
    if (!el->fmt || !el->dec)
        return NULL;

    // Initial or backward jump: seek near the target and flush state.
    if (el->last_pts == MP_NOPTS_VALUE || target < el->last_pts - 1.0) {
        AVStream *st = el->fmt->streams[el->stream_idx];
        avcodec_flush_buffers(el->dec);
        el->ring_len = 0;
        el->eof = false;
        double t = target - 2.0 > 0 ? target - 2.0 : 0;
        int64_t ts = (int64_t)(t * st->time_base.den / st->time_base.num);
        av_seek_frame(el->fmt, el->stream_idx, ts, AVSEEK_FLAG_BACKWARD);
        el->last_pts = MP_NOPTS_VALUE;
    }

    AVPacket *pkt = av_packet_alloc();
    AVFrame *frame = av_frame_alloc();
    AVStream *st = el->fmt->streams[el->stream_idx];
    const struct el_meta_entry *result = NULL;

    if (!pkt || !frame) {
        if (pkt)
            av_packet_free(&pkt);
        if (frame)
            av_frame_free(&frame);
        return NULL;
    }

    while (el->last_pts == MP_NOPTS_VALUE || el->last_pts < target) {
        if (el->eof)
            break;

        // Drain decoder first
        for (;;) {
            int ret = avcodec_receive_frame(el->dec, frame);
            if (ret == AVERROR(EAGAIN) || ret == AVERROR_EOF)
                break;
            if (ret < 0)
                goto done;
            double pts = frame_pts_secs(el->dec, frame, st);
            if (pts != MP_NOPTS_VALUE)
                el->last_pts = pts;
            AVFrameSideData *sd =
                av_frame_get_side_data(frame, AV_FRAME_DATA_DOVI_METADATA);
            if (sd)
                el_push_meta(el, pts, (const AVDOVIMetadata *)sd->buf->data);
            av_frame_unref(frame);
        }

        int ret = av_read_frame(el->fmt, pkt);
        if (ret < 0) {
            el->eof = true;
            avcodec_send_packet(el->dec, NULL); // flush
            continue;
        }
        if (pkt->stream_index == el->stream_idx)
            avcodec_send_packet(el->dec, pkt);
        av_packet_unref(pkt);
    }

done:
    // Nearest entry at or before target; fall back to nearest overall.
    int best = -1;
    double best_diff = 1e9;
    for (int i = 0; i < el->ring_len; i++) {
        double diff = el->ring[i].pts - target;
        if (diff <= 0.05 && -diff < best_diff) {
            best_diff = -diff;
            best = i;
        }
    }
    if (best < 0) {
        for (int i = 0; i < el->ring_len; i++) {
            double diff = fabs(el->ring[i].pts - target);
            if (diff < best_diff) {
                best_diff = diff;
                best = i;
            }
        }
    }
    if (best >= 0) {
        result = &el->ring[best];
        MP_VERBOSE(p, "DoVi EL metadata: target %.3f -> pts %.3f peak=%.1f (ring %d/%d)\n",
                   target, el->ring[best].pts, el->ring[best].tm_peak, best, el->ring_len);
    }
    av_frame_free(&frame);
    av_packet_free(&pkt);
    return result;
}

// ---------------------------------------------------------------------------

static void vf_dovi_process(struct mp_filter *f)
{
    struct priv *p = f->priv;

    if (!mp_pin_can_transfer_data(f->ppins[1], f->ppins[0]))
        return;

    struct mp_frame frame = mp_pin_out_read(f->ppins[0]);

    if (mp_frame_is_signaling(frame)) {
        mp_pin_in_write(f->ppins[1], frame);
        return;
    }

    if (frame.type != MP_FRAME_VIDEO)
        goto passthrough;

    {
        struct mp_image *mpi = frame.data;

        // until 裁剪：目标帧之前的前导帧直通（跳过昂贵的 reshape）
        if (p->opts && p->opts->until >= 0 && mpi->pts >= 0 &&
            mpi->pts < p->opts->until - 0.001)
            goto passthrough;

        if (mpi->params.repr.sys != PL_COLOR_SYSTEM_DOLBYVISION || !mpi->params.repr.dovi) {
            // Dual-stream Profile 7: metadata rides the EL track.
            if (p->opts && p->opts->el_source && !p->el_failed) {
                if (!p->el.fmt && !el_open(p, p->opts->el_source)) {
                    p->el_failed = true;
                    goto passthrough;
                }
                const struct el_meta_entry *ent = el_lookup_entry(p, mpi->pts);
                if (!ent)
                    goto passthrough;
                if (!prepare(p, &ent->meta, &mpi->params.repr.bits, ent->tm_peak))
                    goto passthrough;
            } else {
                goto passthrough; // plain HDR10/SDR
            }
        } else {
            // Single-track P5/P8: metadata attached by the decoder.
            if (!prepare(p, mpi->params.repr.dovi, &mpi->params.repr.bits, 10.0f))
                goto passthrough;
        }

        struct mp_imgfmt_desc desc = mp_imgfmt_get_desc(mpi->imgfmt);
        bool ok_fmt = desc.num_planes == 3 &&
                      (desc.flags & MP_IMGFLAG_YUV_P) &&
                      desc.chroma_xs == 1 && desc.chroma_ys == 1 &&
                      desc.comps[0].size == 16; // planar 420, 16-bit samples

        if (!ok_fmt) {
            if (!p->fmt_warned) {
                MP_WARN(p, "DoVi frame in unsupported format %s, passing through\n",
                        mp_imgfmt_to_name(mpi->imgfmt));
                p->fmt_warned = true;
            }
            goto passthrough;
        }

        struct mp_image *dmpi = mp_image_alloc(mpi->imgfmt, mpi->w, mpi->h);
        if (!dmpi)
            goto passthrough;
        mp_image_copy_attributes(dmpi, mpi);

        if (!reshape_frame(p, mpi, dmpi)) {
            talloc_free(dmpi);
            goto passthrough;
        }

        // Relabel HDR10 PQ bt.2020（显示层转换交 lavfi——16 盘验证链）
        dmpi->params.repr.sys = PL_COLOR_SYSTEM_BT_2020_NC;
        dmpi->params.repr.levels = PL_COLOR_LEVELS_LIMITED;
        dmpi->params.repr.dovi = NULL;
        dmpi->params.color.primaries = PL_COLOR_PRIM_BT_2020;
        dmpi->params.color.transfer = PL_COLOR_TRC_PQ;

        dmpi->pts = mpi->pts;
        dmpi->dovi = NULL;

        MP_VERBOSE(p, "DoVi frame reshaped %dx%d\n", mpi->w, mpi->h);

        talloc_free(mpi);
        mp_pin_in_write(f->ppins[1], MAKE_FRAME(MP_FRAME_VIDEO, dmpi));
        return;
    }

passthrough:
    mp_pin_in_write(f->ppins[1], frame);
}

static void vf_dovi_destroy(struct mp_filter *f)
{
    struct priv *p = f->priv;
    el_close(&p->el);
}

static const struct mp_filter_info filter_dovi_reshape = {
    .name = "dovi_reshape",
    .priv_size = sizeof(struct priv),
    .process = vf_dovi_process,
    .destroy = vf_dovi_destroy,
};

#define OPT_BASE_STRUCT struct dovi_opts
static const m_option_t dovi_opts_list[] = {
    {"el-source", OPT_STRING(el_source)},
    {"until", OPT_DOUBLE(until), OPTDEF_DOUBLE(-1.0)},
    {0}
};

static struct mp_filter *vf_dovi_create(struct mp_filter *parent, void *options)
{
    struct mp_filter *f = mp_filter_create(parent, &filter_dovi_reshape);
    if (!f) {
        talloc_free(options);
        return NULL;
    }

    mp_filter_add_pin(f, MP_PIN_IN, "in");
    mp_filter_add_pin(f, MP_PIN_OUT, "out");

    struct priv *p = f->priv;
    p->log = f->log;
    p->opts = talloc_steal(p, options);
    build_pq_luts(p);

    return f;
}

const struct mp_user_filter_entry vf_dovi_reshape = {
    .desc = {
        .description = "CPU Dolby Vision BL reshaping to HDR10 (zimg path)",
        .name = "dovi_reshape",
        .priv_size = sizeof(struct dovi_opts),
        .options = dovi_opts_list,
    },
    .create = vf_dovi_create,
};
