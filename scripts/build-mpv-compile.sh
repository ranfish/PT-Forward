#!/bin/bash
set -e

cd /work/examples/mpv
rm -rf build

echo "=== MESON SETUP ==="
meson setup build --buildtype=minsize \
  -Dcplayer=true -Dlibmpv=false -Dtests=false -Dbuild-date=false \
  -Dmanpage-build=disabled -Dhtml-build=disabled \
  -Dalsa=disabled -Dpulse=disabled -Djack=disabled -Dpipewire=disabled \
  -Doss-audio=disabled -Dsndio=disabled -Dsdl2-audio=disabled \
  -Dgl=disabled -Dvulkan=disabled -Degl=disabled \
  -Dx11=disabled -Dwayland=disabled -Ddrm=disabled -Dgbm=disabled \
  -Dcaca=disabled -Dsdl2-video=disabled -Dsixel=disabled -Dxv=disabled \
  -Dplain-gl=disabled -Dvdpau=disabled -Dvaapi=disabled \
  -Dshaderc=disabled -Dspirv-cross=disabled \
  -Dcuda-hwaccel=disabled -Dcuda-interop=disabled \
  -Dcdda=disabled -Ddvbin=disabled -Ddvdnav=disabled \
  -Dlibbluray=disabled -Dlibavdevice=disabled \
  -Dlibarchive=disabled \
  -Dlua=disabled -Djavascript=disabled \
  -Drubberband=disabled -Dvapoursynth=disabled \
  -Dcplugins=disabled -Dsdl2-gamepad=disabled \
  -Dlcms2=disabled -Dzimg=enabled \
  -Duchardet=disabled

echo "=== NINJA COMPILE ==="
ninja -C build

echo "=== STRIP ==="
strip build/mpv
echo "Binary size:"
ls -lh build/mpv

echo "=== LDD ==="
ldd build/mpv

echo "=== COPY ==="
cp build/mpv /work/bin/amd64/mpv-new
echo "=== ALL DONE ==="
