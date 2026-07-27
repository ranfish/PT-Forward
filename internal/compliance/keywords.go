package compliance

var (
	AdultKeywords = []string{
		"9KG", "9kg", "色情", "成人内容", "成人影片",
		"AV", "18+", "NSFW", "Adult", "XXX",
		"Porn", "Erotic", "Hentai",
	}

	ForbiddenTransferKeywords = []string{
		"禁转", "禁轉",
		"独占", "獨占",
		"谢绝转载", "謝絕轉載",
		"限时禁转", "限時禁轉",
		"严禁转载", "嚴禁轉載",
		"禁止转载",
		"谢绝搬运",
	}

	ForbiddenGroups = []string{
		"CatEDU",
	}

	SiteCategoryBlacklist = map[string][]string{
		"馒头":  {"adult"},
		"我的PT": {"410"},
	}
)
