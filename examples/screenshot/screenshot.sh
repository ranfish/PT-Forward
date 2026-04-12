#!/bin/bash

rm -rf *.jpg

CONFIG_FILE="./config.conf"

if [[ ! -f "$CONFIG_FILE" ]]; then
    echo "错误: 配置文件 $CONFIG_FILE 不存在"
    exit 1
fi

source "$CONFIG_FILE"

VIDEO_EXTENSIONS="mp4|mkv|avi|mov|wmv|flv|webm|m4v|ts"

if [[ -n "$VIDEO_FILE" && -f "$VIDEO_FILE" ]]; then
    VIDEO_PATH="$VIDEO_FILE"
    echo "使用指定视频文件: $VIDEO_PATH"
elif [[ -n "$VIDEO_DIR" && -d "$VIDEO_DIR" ]]; then
    LARGEST_VIDEO=$(find "$VIDEO_DIR" -type f -regextype posix-extended -iregex ".*\.($VIDEO_EXTENSIONS)$" -printf "%s %p\n" 2>/dev/null | sort -rn | head -n1 | cut -d' ' -f2-)
    
    if [[ -z "$LARGEST_VIDEO" ]]; then
        echo "错误: 在目录 $VIDEO_DIR 中未找到视频文件"
        exit 1
    fi
    
    VIDEO_PATH="$LARGEST_VIDEO"
    echo "自动选择最大视频文件: $VIDEO_PATH"
else
    echo "错误: 请在配置文件中设置有效的 VIDEO_FILE 或 VIDEO_DIR"
    exit 1
fi

VIDEO_SIZE=$(du -h "$VIDEO_PATH" | cut -f1)
echo "文件大小: $VIDEO_SIZE"

HAS_SUBTITLE=0
SUBTITLE_SID=""
SUBTITLE_TYPE=""

subtitle_info=$(ffprobe -v error -select_streams s -show_entries stream=index,codec_name,disposition:stream_tags=language,title -of json "$VIDEO_PATH" 2>/dev/null)

if [[ -n "$subtitle_info" ]]; then
    TEXT_SUBTITLES="subrip|srt|ass|ssa|webvtt|vtt|mov_text"
    GRAPHIC_SUBTITLES="hdmv_pgs_subtitle|pgs|vobsub|dvd_subtitle|dvb_subtitle"
    
    best_ass_sid=0
    best_srt_sid=0
    best_pgs_sid=0
    best_ass_score=0
    best_srt_score=0
    best_pgs_score=0
    sid=1
    
    stream_count=$(echo "$subtitle_info" | jq '.streams | length')
    
    for ((i=0; i<stream_count; i++)); do
        codec=$(echo "$subtitle_info" | jq -r ".streams[$i].codec_name")
        lang=$(echo "$subtitle_info" | jq -r ".streams[$i].tags.language // \"\"" | tr '[:upper:]' '[:lower:]')
        title=$(echo "$subtitle_info" | jq -r ".streams[$i].tags.title // \"\"" | tr '[:upper:]' '[:lower:]')
        comment=$(echo "$subtitle_info" | jq -r ".streams[$i].disposition.comment // 0")
        hearing=$(echo "$subtitle_info" | jq -r ".streams[$i].disposition.hearing_impaired // 0")
        visual=$(echo "$subtitle_info" | jq -r ".streams[$i].disposition.visual_impaired // 0")
        
        if [[ "$comment" == "1" || "$hearing" == "1" || "$visual" == "1" ]]; then
            sid=$((sid + 1))
            continue
        fi
        
        score=0
        sub_type=""
        
        if echo "$codec" | grep -qiE "^($TEXT_SUBTITLES)$"; then
            sub_type="text"
        elif echo "$codec" | grep -qiE "^($GRAPHIC_SUBTITLES)$"; then
            sub_type="graphic"
        fi
        
        if [[ -n "$sub_type" ]]; then
            if [[ "$lang" == "chi" || "$lang" == "zho" || "$lang" == "zh" ]]; then
                score=$((score + 10))
            fi
            
            if [[ "$title" =~ "简" || "$title" =~ "chs" || "$title" =~ "sc" ]]; then
                score=$((score + 5))
            elif [[ "$title" =~ "繁" || "$title" =~ "cht" || "$title" =~ "tc" ]]; then
                score=$((score + 3))
            elif [[ "$title" =~ "中" || "$title" =~ "chinese" ]]; then
                score=$((score + 2))
            fi
            
            if [[ "$codec" == "ass" && $score -gt $best_ass_score ]]; then
                best_ass_score=$score
                best_ass_sid=$sid
            elif [[ "$codec" == "subrip" && $score -gt $best_srt_score ]]; then
                best_srt_score=$score
                best_srt_sid=$sid
            elif [[ "$sub_type" == "graphic" && $score -gt $best_pgs_score ]]; then
                best_pgs_score=$score
                best_pgs_sid=$sid
            fi
        fi
        
        sid=$((sid + 1))
    done
    
    if [[ $best_ass_score -gt 0 ]]; then
        SUBTITLE_SID=$best_ass_sid
        SUBTITLE_TYPE="text"
        HAS_SUBTITLE=1
        echo "检测到 ASS 字幕，选择轨道: $SUBTITLE_SID"
    elif [[ $best_srt_score -gt 0 ]]; then
        SUBTITLE_SID=$best_srt_sid
        SUBTITLE_TYPE="text"
        HAS_SUBTITLE=1
        echo "检测到 SRT 字幕，选择轨道: $SUBTITLE_SID"
    elif [[ $best_pgs_score -gt 0 ]]; then
        SUBTITLE_SID=$best_pgs_sid
        SUBTITLE_TYPE="graphic"
        HAS_SUBTITLE=1
        echo "检测到 PGS 字幕，选择轨道: $SUBTITLE_SID"
    else
        echo "未检测到中文字幕"
    fi
else
    echo "未检测到内嵌字幕"
fi

DURATION=$(ffprobe -v error -show_entries format=duration -of default=noprint_wrappers=1:nokey=1 "$VIDEO_PATH" 2>/dev/null)

if [[ -z "$DURATION" ]]; then
    echo "错误: 无法获取视频时长"
    exit 1
fi

DURATION_INT=${DURATION%.*}

if [[ $DURATION_INT -lt 5 ]]; then
    echo "错误: 视频时长太短"
    exit 1
fi

echo "视频时长: ${DURATION_INT} 秒"

SCREENSHOT_COUNT=5
declare -a TIME_POINTS

GOLDEN_START=$((DURATION_INT * 30 / 100))
GOLDEN_END=$((DURATION_INT * 80 / 100))
MIN_INTERVAL=30

if [[ $GOLDEN_END -le $GOLDEN_START ]]; then
    GOLDEN_START=1
    GOLDEN_END=$((DURATION_INT - 1))
fi

generate_well_distributed_points() {
    local count=$1
    local start=$2
    local end=$3
    local min_interval=$4
    
    local range=$((end - start))
    local interval=$((range / count))
    
    if [[ $interval -lt $min_interval ]]; then
        interval=$min_interval
    fi
    
    local points=()
    local current=$start
    
    for ((i=0; i<count; i++)); do
        local random_offset=$((RANDOM % (interval / 2 + 1)))
        local point=$((current + random_offset))
        
        if [[ $point -gt $end ]]; then
            point=$end
        fi
        
        points+=($point)
        current=$((current + interval))
    done
    
    echo "${points[@]}"
}

TIME_POINTS=($(generate_well_distributed_points $SCREENSHOT_COUNT $GOLDEN_START $GOLDEN_END $MIN_INTERVAL))

echo "截取时间点: ${TIME_POINTS[*]}"

for ((i=0; i<SCREENSHOT_COUNT; i++)); do
    TIMESTAMP=${TIME_POINTS[$i]}
    
    total_seconds=$TIMESTAMP
    m=$((total_seconds % 60))
    temp=$((total_seconds / 60))
    h=$((temp / 60))
    min=$((temp % 60))
    time_str=$(printf "%02dh%02dm%02ds" $h $min $m)
    
    OUTPUT_FILE="s$((i+1))_${time_str}.jpg"
    
    echo "正在截取第 $((i+1)) 张图片，时间点: ${TIMESTAMP}秒 ($time_str)..."
    
    if [[ $HAS_SUBTITLE -eq 1 ]]; then
        mpv --vo=image --ao=null --no-audio --start="$TIMESTAMP" --frames=1 --sid="$SUBTITLE_SID" --sub-visibility=yes --blend-subtitles=yes --no-terminal "$VIDEO_PATH" 2>/dev/null
        
        if [[ -f "00000001.jpg" ]]; then
            mv "00000001.jpg" "$OUTPUT_FILE"
            echo "已保存: $OUTPUT_FILE"
        else
            echo "截取失败: $OUTPUT_FILE"
        fi
    else
        mpv --vo=image --ao=null --no-audio --start="$TIMESTAMP" --frames=1 --sid=no --no-terminal "$VIDEO_PATH" 2>/dev/null
        
        if [[ -f "00000001.jpg" ]]; then
            mv "00000001.jpg" "$OUTPUT_FILE"
            echo "已保存: $OUTPUT_FILE"
        else
            echo "截取失败: $OUTPUT_FILE"
        fi
    fi
done

echo "截图完成！"
