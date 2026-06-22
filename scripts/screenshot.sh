#!/bin/bash
set -e

VIDEO="$1"
OUTDIR="$(dirname "$VIDEO")"
MPV="${MPV_PATH:-mpv}"
FFPROBE="${FFPROBE_PATH:-ffprobe}"
COUNT="${SCREENSHOT_COUNT:-5}"
MIN_INTERVAL="${SCREENSHOT_MIN_INTERVAL:-30}"
QUALITY="${SCREENSHOT_QUALITY:-85}"

# 获取时长
DURATION=$($FFPROBE -v error -show_entries format=duration -of default=noprint_wrappers=1:nokey=1 "$VIDEO" 2>/dev/null)
DURATION=${DURATION%.*}
if [ -z "$DURATION" ] || [ "$DURATION" -lt 5 ]; then
  echo "错误: 无法获取视频时长"; exit 1
fi
echo "时长: ${DURATION}秒"

# 检测 HDR（smpte2084=PQ, arib-std-b67=HLG）
TRANSFER=$($FFPROBE -v error -select_streams v:0 -show_entries stream=color_transfer -of default=noprint_wrappers=1:nokey=1 "$VIDEO" 2>/dev/null)
IS_HDR=false
if [ "$TRANSFER" = "smpte2084" ] || [ "$TRANSFER" = "arib-std-b67" ]; then
  IS_HDR=true
fi
echo "transfer: $TRANSFER, HDR: $IS_HDR"

# 检测中文字幕（ASS > SRT > PGS 优先级，匹配 screenshot_v2.py 策略）
SUB_SID=""
SUB_INFO=$($FFPROBE -v error -select_streams s \
  -show_entries stream=index,codec_name:stream_tags=language,title \
  -of csv=p=0 "$VIDEO" 2>/dev/null)
BEST_ASS_SID=0; BEST_ASS_SCORE=0
BEST_SRT_SID=0; BEST_SRT_SCORE=0
BEST_PGS_SID=0; BEST_PGS_SCORE=0
SID=1
while IFS=',' read -r idx codec lang title; do
  lang=$(echo "$lang" | tr '[:upper:]' '[:lower:]')
  title=$(echo "$title" | tr '[:upper:]' '[:lower:]')
  score=0
  case "$lang" in chi|zho|zh) score=$((score+10)) ;; esac
  case "$title" in *简*|*chs*|*sc*) score=$((score+5)) ;; *繁*|*cht*|*tc*) score=$((score+3)) ;; *中*|*chinese*) score=$((score+2)) ;; esac
  codec=$(echo "$codec" | tr '[:upper:]' '[:lower:]')
  if echo "$codec" | grep -qi "ass\|ssa"; then
    [ "$score" -gt "$BEST_ASS_SCORE" ] && BEST_ASS_SCORE=$score && BEST_ASS_SID=$SID
  elif echo "$codec" | grep -qi "subrip\|srt"; then
    [ "$score" -gt "$BEST_SRT_SCORE" ] && BEST_SRT_SCORE=$score && BEST_SRT_SID=$SID
  else
    [ "$score" -gt "$BEST_PGS_SCORE" ] && BEST_PGS_SCORE=$score && BEST_PGS_SID=$SID
  fi
  SID=$((SID+1))
done <<< "$SUB_INFO"
if [ "$BEST_ASS_SCORE" -gt 0 ]; then SUB_SID=$BEST_ASS_SID
elif [ "$BEST_SRT_SCORE" -gt 0 ]; then SUB_SID=$BEST_SRT_SID
elif [ "$BEST_PGS_SCORE" -gt 0 ]; then SUB_SID=$BEST_PGS_SID
fi
[ -n "$SUB_SID" ] && [ "$SUB_SID" -gt 0 ] && echo "中文字幕 sid=$SUB_SID" || echo "未检测到中文字幕"

# 黄金区间 30%-80% + 随机偏移
GOLDEN_START=$((DURATION * 30 / 100))
GOLDEN_END=$((DURATION * 80 / 100))
RANGE=$((GOLDEN_END - GOLDEN_START))
INTERVAL=$((RANGE / COUNT))
[ "$INTERVAL" -lt "$MIN_INTERVAL" ] && INTERVAL=$MIN_INTERVAL

# 构建公共参数
COMMON_ARGS="--no-config --no-terminal --vo=image --ao=null --no-audio"
# HDR 视频加 mobius tone-mapping（改善高光过曝）
if [ "$IS_HDR" = "true" ]; then
  COMMON_ARGS="$COMMON_ARGS --vf=lavfi=[tonemap=mobius]"
fi

TMPDIR=$(mktemp -d)
trap "rm -rf $TMPDIR" EXIT

CURRENT=$GOLDEN_START
for i in $(seq 1 $COUNT); do
  OFFSET=$((RANDOM % (INTERVAL / 2 + 1)))
  POINT=$((CURRENT + OFFSET))
  [ "$POINT" -gt "$GOLDEN_END" ] && POINT=$GOLDEN_END
  H=$((POINT / 3600)); M=$(((POINT % 3600) / 60)); S=$((POINT % 60))
  TIME_STR=$(printf "%02dh%02dm%02ds" $H $M $S)
  OUTFILE="${OUTDIR}/s${i}_${TIME_STR}.jpg"
  echo ""
  echo "截图 ${i}/${COUNT}: ${POINT}s"

  SUB_ARGS="--sid=no"
  if [ -n "$SUB_SID" ] && [ "$SUB_SID" -gt 0 ]; then
    SUB_ARGS="--sid=$SUB_SID --sub-visibility=yes --blend-subtitles=yes"
  fi

  $MPV $COMMON_ARGS \
    --frames=1 --start=${POINT} \
    --vo-image-format=jpg --vo-image-jpeg-quality=$QUALITY \
    --vo-image-outdir="$TMPDIR" $SUB_ARGS "$VIDEO" 2>&1

  if [ -f "$TMPDIR/00000001.jpg" ]; then
    mv "$TMPDIR/00000001.jpg" "$OUTFILE"
    ls -lh "$OUTFILE"
  else
    echo "  FAILED"
  fi
  CURRENT=$((CURRENT + INTERVAL))
done

echo ""
echo "=== 完成 ==="
ls -lh "${OUTDIR}"/s*_*.jpg 2>/dev/null
