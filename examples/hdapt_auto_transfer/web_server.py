import hashlib
import os
from functools import wraps

import yaml
from flask import Flask, flash, redirect, render_template, request, session, url_for


CONFIG_PATH = os.environ.get("CONFIG_PATH", "config.yaml")


def load_config():
    with open(CONFIG_PATH, "r", encoding="utf-8") as f:
        return yaml.safe_load(f)


def get_web_password(config):
    return os.environ.get("WEB_UI_PASSWORD") or config.get("web_ui", {}).get("password", "")


def build_secret_key(config):
    explicit_secret = os.environ.get("WEB_UI_SECRET_KEY")
    if explicit_secret:
        return explicit_secret

    password = get_web_password(config)
    seed = f"{CONFIG_PATH}|{password or 'pt-auto-transfer'}"
    return hashlib.sha256(seed.encode("utf-8")).hexdigest()


app = Flask(__name__)
app.secret_key = build_secret_key(load_config())


def is_authenticated(config):
    password = get_web_password(config)
    if not password:
        return True
    return session.get("web_ui_authed") is True


def require_login(view):
    @wraps(view)
    def wrapped(*args, **kwargs):
        config = load_config()
        if not is_authenticated(config):
            flash("请先输入 Web UI 密码。", "error")
            return redirect(url_for("login", next=request.path))
        return view(*args, **kwargs)

    return wrapped


def save_config(config):
    app.secret_key = build_secret_key(config)
    with open(CONFIG_PATH, "w", encoding="utf-8") as f:
        yaml.safe_dump(config, f, allow_unicode=True, sort_keys=False)


@app.route("/login", methods=["GET", "POST"])
def login():
    config = load_config()
    password = get_web_password(config)
    next_url = request.args.get("next") or request.form.get("next") or url_for("index")

    if not password:
        session["web_ui_authed"] = True
        return redirect(next_url)

    if request.method == "POST":
        provided = request.form.get("password", "")
        if provided == password:
            session["web_ui_authed"] = True
            flash("登录成功。", "success")
            return redirect(next_url)
        flash("密码错误。", "error")

    return render_template("login.html", next_url=next_url)


@app.route("/logout")
def logout():
    session.pop("web_ui_authed", None)
    flash("已退出登录。", "success")
    return redirect(url_for("login"))


@app.route("/")
@require_login
def index():
    config = load_config()
    monitor_urls_str = "\n".join(config["sites"]["ttg"].get("monitor_urls", []))
    mt_monitor_urls_str = "\n".join(config["sites"].get("mteam", {}).get("monitor_urls", []))
    return render_template(
        "index.html",
        config=config,
        monitor_urls_str=monitor_urls_str,
        mt_monitor_urls_str=mt_monitor_urls_str,
    )


@app.route("/save", methods=["POST"])
@require_login
def save():
    try:
        config = load_config()

        config["sites"]["ttg"]["cookie"] = request.form.get("ttg_cookie")
        config["sites"]["hdarea"]["cookie"] = request.form.get("hda_cookie")

        if "mteam" not in config["sites"]:
            config["sites"]["mteam"] = {}
        config["sites"]["mteam"]["api_key"] = request.form.get("mt_api_key")
        config["sites"]["mteam"]["free_only"] = request.form.get("mt_free_only") == "on"

        raw_mt_urls = request.form.get("mt_monitor_urls", "")
        config["sites"]["mteam"]["monitor_urls"] = [url.strip() for url in raw_mt_urls.split("\n") if url.strip()]

        if "monitor_categories" not in config["sites"]["mteam"]:
            config["sites"]["mteam"]["monitor_categories"] = ["419"]

        raw_urls = request.form.get("monitor_urls", "")
        config["sites"]["ttg"]["monitor_urls"] = [url.strip() for url in raw_urls.split("\n") if url.strip()]

        config["metadata_api"]["imdb_to_douban"] = request.form.get("meta_api_url")

        config["qbittorrent"]["host"] = request.form.get("qb_host")
        config["qbittorrent"]["username"] = request.form.get("qb_username")
        config["qbittorrent"]["password"] = request.form.get("qb_password")
        config["qbittorrent"]["save_path"] = request.form.get("qb_path")
        config["qbittorrent"]["max_global_upload_speed_mb"] = float(request.form.get("qb_global_speed", 90))
        config["qbittorrent"]["max_torrent_upload_speed_mb"] = float(request.form.get("qb_torrent_speed", 90))
        config["qbittorrent"]["use_super_seeding"] = request.form.get("qb_super_seeding") == "on"

        config["concurrency"]["max_active_downloads"] = int(request.form.get("max_dl", 3))
        config["concurrency"]["max_active_uploads"] = int(request.form.get("max_up", 100))

        config["cleanup_rules"]["max_seed_time_hours"] = int(request.form.get("max_seed_time", 48))
        config["cleanup_rules"]["min_seeders_for_deletion"] = int(request.form.get("min_seeders", 5))
        config["cleanup_rules"]["min_ratio_for_deletion"] = float(request.form.get("min_ratio", 1.1))
        
        low_speed_combo = request.form.get("low_speed_combo", "20/10")
        try:
            if "/" in low_speed_combo:
                kb_str, time_str = low_speed_combo.split("/")
                config["cleanup_rules"]["low_speed_threshold_kb"] = int(kb_str.strip())
                config["cleanup_rules"]["low_speed_time_minutes"] = int(time_str.strip())
        except Exception as e:
            print(f"Error parsing low_speed_combo: {e}")

        config["settings"]["max_ttl_hours"] = int(request.form.get("max_ttl", 24))
        config["settings"]["check_interval"] = int(request.form.get("check_interval", 600))
        config["settings"]["min_free_space_gb"] = int(request.form.get("min_free_space", 30))
        config["settings"]["max_torrent_size_gb"] = float(request.form.get("max_size", 0))

        remote_p = request.form.get("remote_path", "").strip()
        local_p = request.form.get("local_path", "").strip()
        if remote_p and local_p:
            config["settings"]["path_mapping"] = {remote_p: local_p}
        elif "path_mapping" in config["settings"]:
            del config["settings"]["path_mapping"]

        web_ui = config.setdefault("web_ui", {})
        new_password = request.form.get("web_ui_password", "").strip()
        web_ui["password"] = new_password
        session["web_ui_authed"] = True

        save_config(config)
        flash("所有配置已保存。", "success")
    except Exception as e:
        flash(f"保存发生错误: {str(e)}", "error")

    return redirect(url_for("index"))


@app.route("/clear_cache", methods=["POST"])
@require_login
def clear_cache():
    try:
        with open(".clear_cache_flag", "w", encoding="utf-8") as f:
            f.write("clear")
        flash("清除缓存指令已下发，主程序将在下一个周期清空缓存并重新开始。", "success")
    except Exception as e:
        flash(f"下发清除缓存指令失败: {str(e)}", "error")
    return redirect(url_for("index"))

@app.route("/api/logs")
@require_login
def api_logs():
    try:
        log_path = "pt_transfer.log"
        if not os.path.exists(log_path):
            return {"logs": "暂无日志记录...\n"}
        with open(log_path, "r", encoding="utf-8") as f:
            lines = f.readlines()
            return {"logs": "".join(lines[-300:])}
    except Exception as e:
        return {"logs": f"无法读取日志: {e}"}

if __name__ == "__main__":
    config = load_config()
    port = config.get("web_ui", {}).get("port", 8888)
    app.run(host="0.0.0.0", port=port)
