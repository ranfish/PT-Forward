package screenshot

type Config struct {
	MpvPath     string
	Count       int
	MinInterval int
	JPEGQuality int
	// Format §59.462: 截图输出格式（jpeg 默认/png 可选——海豹站规
	// PNG；默认 jpeg 存量零变化）。空值按 jpeg 处理（防御旧配置）。
	Format string
}
