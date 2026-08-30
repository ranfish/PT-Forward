#!/usr/bin/env python3
"""§59.151 幸运专线全量端到端推演：243 ready 种子 → 组装幸运表单 → 官方 pre-audit 批量验证"""
import json, time, urllib.request, sys

API = "http://127.0.0.1:8765/api/v1"
LUCK_API = "https://pt.luckpt.de/api/auto-audit/pre-audit"
OPENER = urllib.request.build_opener()

# 幸运站映射表（§59.149/150 实测定案）
TYPE_MAP = {
    'category.movie': ('401', '电影'), 'category.tv_series': ('402', '电视剧'),
    'category.animation': ('405', '动画'), 'category.tv_shows': ('410', '综艺'),
    'category.documentary': ('411', '纪录片'), 'category.short_drama': ('413', '短剧'),
    'category.sports': ('412', '体育'), 'category.mv': ('406', 'MV'),
    'category.music': ('408', '音乐'), 'category.other': ('409', '其他'),
    'category.hq_audio': ('408', '音乐'), 'category.concert': ('406', 'MV'),
    'category.audiobook': ('408', '音乐'), 'category.game': ('409', '其他'),
    'category.software': ('409', '其他'), 'category.ebook': ('409', '其他'),
    'category.study': ('409', '其他'), 'category.comic': ('405', '动画'),
    'category.adult': (None, None),
}
CODEC_MAP = {'x265': ('6', 'H.265/HEVC'), 'h265': ('6', 'H.265/HEVC'), 'HEVC': ('6', 'H.265/HEVC'), 'x264': ('1', 'H.264/AVC'),
             'h264': ('1', 'H.264/AVC'), 'av1': ('2', 'AV1'), 'vc1': ('3', 'VC-1'),
             'mpeg2': ('4', 'MPEG-2'), 'xvid': ('12', 'MPEG-4/XviD'), 'mpeg4': ('12', 'MPEG-4/XviD')}
STD_MAP = {'2160p': ('6', '4K/2160p'), '1080p': ('1', '1080p/1080i'), '720p': ('3', '720p/720i'),
           '480p': ('4', '480p/480i'), '4320p': ('7', '8K/4320p'), '1440p': ('5', '2K/1440p')}
TEAM_MAP = {'FRDS': ('9', 'FRDS')}  # 其它组→Other 5
AUDIO_COMBO = {  # audio_codec × audio_tech → 幸运 audiocodec_sel
    ('TrueHD', 'Atmos'): ('11', 'TrueHD Atmos'), ('TrueHD', ''): ('14', 'TrueHD'),
    ('DDP', ''): ('12', 'DDP/E-AC3'), ('DDP', 'Atmos'): ('12', 'DDP/E-AC3'),
    ('DD+', ''): ('12', 'DDP/E-AC3'), ('DD+', 'Atmos'): ('12', 'DDP/E-AC3'),
    ('DTS-HD MA', ''): ('16', 'DTS-HD MA'), ('DTS', 'X'): ('15', 'DTS:X'),
    ('DTS', ''): ('3', 'DTS'), ('DTS-ES', ''): ('16', 'DTS-HD MA'), ('FLAC', ''): ('1', 'FLAC'), ('AAC', ''): ('6', 'AAC'),
    ('AC3', ''): ('8', 'DD/AC3'), ('DD', ''): ('8', 'DD/AC3'), ('APE', ''): ('2', 'APE'),
    ('LPCM', ''): ('13', 'LPCM'), ('TrueHD', 'DTS:X'): ('11', 'TrueHD Atmos'),
}
TAG_MAP = {  # standard_key → 幸运(id, name)；§59.151 修正版
    'dolby_vision': ('20', 'Dolby Vision'), 'hdr10': ('19', 'HDR10'), 'hdr10_plus': ('18', 'HDR10+'),
    'chinese_subtitle': ('6', '中字'), 'guoyu_audio': ('5', '国语'), 'chinese_audio': ('5', '国语'), 'cantonese_audio': ('14', '粤语'),
    'diy': ('4', 'DIY'), 'complete': ('10', '完结'), 'ongoing': ('9', '连载'),
    'collection': ('17', '合集'), 'big_pack': ('11', '大包'), 'effects_subtitle': ('16', '特效'),
    'vivid_hdr': ('21', '菁彩HDR'),
}

def medium_of(d, encode=None):
    spec = (d.get('specification') or '').lower()
    st = d.get('source_type') or ''
    title = d.get('title') or ''
    # §59.151 终版: 幸运按 title 字符判媒介（用户判据——WEB-DL 字样→WEB-DL;
    # 无 WEB-DL 字样判 Encode）。title 字符优先，spec 词次之（title 保真 spec）
    if 'WEB-DL' in title or spec in ('web-dl', 'webdl'):
        return ('11', 'WEB-DL')
    if 'WEBRip' in title or spec == 'webrip':
        return ('7', 'Encode')
    # 碟源/无 spec：MI Writing library 铁证驱动
    if encode is True and spec not in ('bdrip', 'dvdrip'):
        return ('7', 'Encode')
    if encode is False and spec in ('web-dl', 'webdl', 'webrip'):
        return ('11', 'WEB-DL')
    if spec == 'remux': return ('3', 'Remux')
    if spec in ('web-dl', 'webdl'): return ('11', 'WEB-DL')
    if spec == 'webrip': return ('11', 'WEB-DL')
    if spec == 'hdtv': return ('5', 'HDTV')
    if spec in ('bdrip', 'dvdrip'): return ('7', 'Encode')
    if st in ('UHD Blu-ray', 'Blu-ray', '3D Blu-ray'): return ('10', 'UHD Blu-ray') if 'UHD' in st else ('1', 'Blu-ray')
    if st in ('UHD BluRay', 'BluRay', '3D BluRay'): return ('7', 'Encode')
    if st == 'HDDVDRip' or 'DVD' in st: return ('6', 'DVD')
    return (None, None)

def audit_body(d):
    t = TYPE_MAP.get(d.get('category') or '', (None, None))
    q = {}
    m = medium_of(d, d.get('encode'))
    if m[0]: q['medium'] = {'id': m[0], 'name': m[1]}
    c = CODEC_MAP.get(d.get('video_codec') or '', (None, None))
    if c[0]: q['codec'] = {'id': c[0], 'name': c[1]}
    s = STD_MAP.get(d.get('resolution') or '', (None, None))
    if s[0]: q['standard'] = {'id': s[0], 'name': s[1]}
    a = AUDIO_COMBO.get((d.get('audio_codec') or '', d.get('audio_tech') or ''))
    if not a:
        a = AUDIO_COMBO.get((d.get('audio_codec') or '', ''), (None, None))
    if a and a[0]: q['audiocodec'] = {'id': a[0], 'name': a[1]}
    tm = TEAM_MAP.get(d.get('release_group') or '')
    if tm: q['team'] = {'id': tm[0], 'name': tm[1]}
    tags = []
    for tkey in (d.get('tags') or []):
        hit = TAG_MAP.get(tkey)
        if hit: tags.append({'id': hit[0], 'name': hit[1]})
    shots = d.get('screenshots') or []
    descr = (d.get('description') or '')[:6000]
    if shots:
        descr = '\n'.join(f'[img]{u}[/img]' for u in shots[:8]) + '\n' + descr
    if d.get('imdb_url'):
        descr += f"\n[url={d['imdb_url']}]IMDb[/url]"
    if d.get('douban_url'):
        descr += f"\n[url={d['douban_url']}]豆瓣[/url]" 
    return {
        'name': d.get('title') or '', 'small_descr': d.get('subtitle') or '',
        'imdb_url': d.get('imdb_url') or '', 'description': descr,
        'technical_info': d.get('mediainfo') or '',
        'type': {'id': t[0], 'name': t[1], 'mode': '4'} if t[0] else None,
        'quality': q, 'tags': tags,
        'page_url': 'https://pt.luckpt.de/upload.php',
    }

NO_PROXY = urllib.request.build_opener(urllib.request.ProxyHandler({}))

def api(path, token, method='GET', data=None):
    req = urllib.request.Request(API + path, method=method,
        headers={'Authorization': 'Bearer ' + token, 'Content-Type': 'application/json'})
    body = json.dumps(data).encode() if data else None
    r = NO_PROXY.open(req, body, timeout=60)
    return json.loads(r.read())

def pre_audit(body, cookie):
    data = dict(body)
    data['user_info'] = {'id': 10136, 'username': 'ranfish', 'class': 10}
    data['export_time'] = '2026-08-30T00:00:00.000Z'
    req = urllib.request.Request(LUCK_API, data=json.dumps(data).encode(), method='POST',
        headers={'Content-Type': 'application/json', 'Accept': 'application/json',
                 'X-Requested-With': 'XMLHttpRequest',
                 'User-Agent': 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36',
                 'Cookie': cookie})
    resp = OPENER.open(req, timeout=40)
    return json.loads(resp.read())

def main():
    cookie = open('/tmp/lk.ck').read().strip()
    token = api('/auth/login', '', 'POST', {'username': 'admin', 'password': 'Xsy2026!'})['data']['accessToken']
    lst = api('/publish/seeds?ready=true&page_size=300', token)['data']['items']
    print(f"ready 簇: {len(lst)}", flush=True)
    results, errors = [], []
    for i, it in enumerate(lst):
        name = it['name'][:40]
        try:
            det = api(f"/publish/seeds/{it['hash']}", token)['data']
            body = audit_body(det)
            missing = []
            if not body['type']: missing.append('type')
            if 'medium' not in body['quality']: missing.append('medium')
            if 'codec' not in body['quality']: missing.append('codec')
            if 'standard' not in body['quality']: missing.append('standard')
            if 'audiocodec' not in body['quality']: missing.append('audiocodec')
            try:
                r = pre_audit(body, cookie)['data']
                results.append({'name': name, 'passed': r.get('passed'), 'score': r.get('totalScore'),
                                'errors': [(d['errorCode'], d['message'][:80]) for d in (r.get('details') or []) if d['level'] == 'ERROR'],
                                'warnings': [(d['errorCode'], d['message'][:60]) for d in (r.get('details') or []) if d['level'] == 'WARNING'],
                                'missing': missing,
                                'cat': det.get('category'), 'st': det.get('source_type'), 'spec': det.get('specification'),
                                'ac': det.get('audio_codec'), 'at': det.get('audio_tech'), 'vc': det.get('video_codec')})
                st = '✓' if r.get('passed') else '✗'
                print(f"[{i+1}/{len(lst)}] {st} {r.get('totalScore')} {name}", flush=True)
            except Exception as e:
                errors.append({'name': name, 'stage': 'pre-audit', 'err': str(e)[:120]})
                print(f"[{i+1}/{len(lst)}] API-ERR {name}: {str(e)[:60]}", flush=True)
            time.sleep(1.2)
        except Exception as e:
            errors.append({'name': name, 'stage': 'detail', 'err': str(e)[:120]})
        if (i + 1) % 20 == 0:
            json.dump({'results': results, 'errors': errors}, open('/tmp/dryrun.json', 'w'), ensure_ascii=False)
    json.dump({'results': results, 'errors': errors}, open('/tmp/dryrun.json', 'w'), ensure_ascii=False)
    ok = sum(1 for r in results if r['passed'])
    print(f"\n═══ 汇总: 通过 {ok}/{len(results)} | 调用异常 {len(errors)} ═══")

if __name__ == '__main__':
    main()
