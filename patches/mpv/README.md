# mpv DoVi CPU reshaping 补丁（PT-Forward 定制）

## 背景（§59.300）

DoVi Profile 7 双流原盘（BL 4K + EL 1080p 同 m2ts，RPU 挂 EL 流）在无 GPU 环境下
截图灰屏/花屏的两个根因：

1. **mpv vo=image（zimg）无 DoVi 支持**：libplacebo 的 dovi reshape 只在 gpu/gpu-next
   VO（需 GPU）；软件路径把 BL 当普通 bt2020/pq 直转出现白化（实测 JPEG mean 252）
2. **open-GOP seek concealment**：mpv 对 m2ts 的 seek 落点首 1-8 帧常为参考缺失的
   RASL 帧，解码器输出均匀灰（mean 512 std≈0）

## 方案

- `vf_dovi_reshape`：mpv 内建 vf，纯 CPU 实现 libplacebo 的 dovi reshape 数学链
  （curves poly/MMR → nonlinear 矩阵+offset 折叠 → PQ EOTF → LMS→RGB → PQ OETF），
  输出重标记为标准 HDR10（bt2020nc/pq/limited），下游 zimg 正常转换
- `el-source` 选项：vf 内部用 lavc 打开同文件 EL 流解码泵取 RPU side data
  （像素数据不使用，1080p 解码开销可忽略；libplacebo<364 的 FEL 拒绝守卫被
  自写 AVDOVIMetadata→pl_dovi_metadata 映射绕过）
- 截图引擎侧（internal/publish/screenshot.go）：双 HEVC 流检测 → ffprobe 定位目标点
  前最近 IDR → mpv --start=<IDR> --frames=<offset+2> 取末帧（跳过 concealment 区）

## 构建（29 开发机）

```bash
# 1. 准备 mpv v0.41.0 源码（examples/mpv，checkout v0.41.0 tag）
# 2. 应用注册补丁 + 拷贝 vf 源码
cd examples/mpv
git apply ../../patches/mpv/mpv-v0.41.0-registration.patch
cp ../../patches/mpv/vf_dovi_reshape.c video/filter/
# 3. docker 构建环境 + 编译（产出 bin/amd64/mpv-new 随主镜像分发）
sg docker -c "docker build -f docker/Dockerfile.mpv-build -t mpv-build docker/"
sg docker -c "docker run --rm -v $PWD:/work mpv-build bash /work/scripts/build-mpv-compile.sh"
```

## 版本约束

- mpv: v0.41.0（master 要求 libplacebo>=7.360 而 Debian trixie=7.349）
- libplacebo: 7.349（PL_API_VER 349；pl_dovi_metadata 无 nlq 字段——FEL 残差不合成，
  实测新片 EL 残差≈0 无损；真 P5/P8 内嵌 RPU 单流也走本 vf）
- FFmpeg: trixie libavcodec 7.1（hevcdec NAL62→DOVI_METADATA side data 导出）

## 用法

```
mpv --vf=dovi_reshape=el-source='/path/disc.m2ts' ...   # 双流 P7
mpv --vf=dovi_reshape ...                              # 单流 P5/P8（帧自带元数据）
```

非 DoVi 帧自动 passthrough，可无条件启用。
