package extract

// §56.11 extractMediaInfo：MediaInfo/BDInfo 文本提取。
// 2b.4 阶段 stub（返回空字符串），2b.5 完善实现：
//   - 4 层候选源（<pre>/[quote]/<div class="codemain">/<fieldset>）
//   - 合法性检测（强关键字 ≥2 / 弱关键字 ≥2 + VIDEO|AUDIO）
//   - 前缀去污染（General anchor / DISC INFO anchor）
//   - 空行压缩
func (p *PublicExtractor) extractMediaInfo(descrHTML, descrBBCode string) (mediainfo, bdinfo string) {
	return "", ""
}
