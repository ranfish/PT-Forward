package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"

	"github.com/ranfish/pt-forward/internal/config"
	"github.com/ranfish/pt-forward/internal/crypto"
	"github.com/ranfish/pt-forward/internal/model"

	"go.uber.org/zap"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func main() {
	cfg, _ := config.Load("configs/config.yaml", zap.Must(zap.NewProduction()))
	db, _ := gorm.Open(sqlite.Open("data/pt-forward.db"), &gorm.Config{Logger: logger.Discard})
	enc, _ := crypto.NewCredentialEncryptor(cfg.Security.EncryptionKey)
	var cl model.ClientConfig
	db.Where("name = ?", "AutoDUT").First(&cl)
	pw, _ := enc.Decrypt(cl.Password)
	jar, _ := cookiejar.New(nil)
	client := &http.Client{Jar: jar, Transport: &http.Transport{Proxy: nil}}
	form := url.Values{"username": {cl.Username}, "password": {pw}}
	resp, err := client.PostForm("http://10.0.0.242:9090/api/v2/auth/login", form)
	if err != nil {
		fmt.Println("ERR:", err)
		return
	}
	io.ReadAll(resp.Body)
	resp.Body.Close()
	// 全量分页
	all := []map[string]interface{}{}
	for offset := 0; ; offset += 5000 {
		req, _ := http.NewRequest("GET", fmt.Sprintf("http://10.0.0.242:9090/api/v2/torrents/info?limit=5000&offset=%d", offset), nil)
		r2, err := client.Do(req)
		if err != nil {
			return
		}
		raw, _ := io.ReadAll(r2.Body)
		r2.Body.Close()
		var ts []map[string]interface{}
		json.Unmarshal(raw, &ts)
		if len(ts) == 0 {
			break
		}
		all = append(all, ts...)
	}
	fmt.Println("AutoDUT 种子总数:", len(all))
	tid := map[string]int{}
	for _, t := range all {
		c, _ := t["comment"].(string)
		c = strings.TrimSpace(c)
		if c == "" {
			tid["(空)"]++
			continue
		}
		tid["非空"]++
	}
	fmt.Println("comment 分布:", tid)
}
