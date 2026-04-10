FROM python:3.9-slim

# 安装系统级媒体依库
RUN apt-get update && apt-get install -y \
    libmediainfo0v5 \
    mediainfo \
    ffmpeg \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app

# 复制依赖并安装
COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt

# 复制整个项目
COPY . .

# 强制清除 Python 字节码缓存，防止旧 .pyc 文件导致方法缺失
RUN find /app -type d -name "__pycache__" -exec rm -rf {} + 2>/dev/null || true \
    && find /app -name "*.pyc" -delete 2>/dev/null || true

# 保险起见，确保镜像内部的 transfer_state.json 是文件而不是文件夹，防止挂载时被识别为目录
RUN rm -rf /app/transfer_state.json && touch /app/transfer_state.json

EXPOSE 8888

CMD ["python", "main.py"]
