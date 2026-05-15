package screenshot

// TODO(v2): implement screenshot generation via mpv/ffmpeg pipe.
// The Config struct and Pipeline.SetScreenshotConfig exist as scaffolding;
// the actual capture, resize, and upload logic will be added when
// the screenshot feature is prioritized.
type Config struct {
	MpvPath     string
	Count       int
	MinInterval int
	JPEGQuality int
}
