# `scripts/test` 演示造数说明

## 精确回滚与轻量重造

如果你已经把演示数据打得过大，或者想把点击量重新控制到更适合看曲线的范围，优先使用下面这条命令：

```powershell
.\scripts\test\run_chart_demo_reset.ps1
```

它默认会对下面这 10 个短链执行一轮“清理错误窗口 + 轻量重造”：

- `52KD2a`
- `nvGQWJQ0`
- `lV86cWyd`
- `Doubao`
- `13dsw4`
- `89sdss`
- `7d2aPPN4`
- `i652zhFD6xI`
- `leNMP4LV`
- `gIYc1lNwbGO`

默认目标值：

- `52KD2a=820`
- `nvGQWJQ0=620`
- `lV86cWyd=430`
- `Doubao=320`
- `13dsw4=240`
- `89sdss=160`
- `7d2aPPN4=110`
- `i652zhFD6xI=80`
- `leNMP4LV=70`
- `gIYc1lNwbGO=50`

支持的模式：

```powershell
# 只清理错误窗口，并从剩余原始日志重建 click_count / stats_*
.\scripts\test\run_chart_demo_reset.ps1 -CleanupOnly

# 已经插入了原始日志，但聚合没同步时，只重建聚合
.\scripts\test\run_chart_demo_reset.ps1 -RebuildOnly

# 跳过清理，只按目标值补一轮轻量数据
.\scripts\test\run_chart_demo_reset.ps1 -SeedOnly
```

如果要自定义短链或目标值，直接覆盖参数：

```powershell
.\scripts\test\run_chart_demo_reset.ps1 `
  -ShortCodes "52KD2a,nvGQWJQ0,lV86cWyd" `
  -Targets "52KD2a=700,nvGQWJQ0=520,lV86cWyd=360"
```

这条脚本底层调用的是：

```powershell
go run .\cmd\curate-demo-data --short-codes "..." --targets "..."
```

---
# `scripts/test` 婕旂ず閫犳暟璇存槑

杩欎釜鐩綍鐜板湪涓昏鏈嶅姟涓ょ被鐢ㄩ€旓細

1. 缁欑粺璁￠〉琛ヤ竴鎵光€滀笂涓湀 + 鏈湀涓婃棳鈥濈殑鍘嗗彶鏁版嵁锛岃鏃堕棿绾挎洿鑷劧銆?2. 缁欌€滀粖澶┾€濆啀鎵撲竴浜涚湡瀹炶闂祦閲忥紝璁╄繎鏈熻秼鍔裤€佹潵婧愩€佸湴鍩熴€佺幆澧冨垎甯冪户缁彉鍖栥€?
鎺ㄨ崘浣犵洿鎺ヤ娇鐢ㄤ竴閿剼鏈細

- `scripts/test/run_demo_seed.ps1`

瀹冧細鎶婏細

- 鍘嗗彶鏁版嵁鐩村啓鏁版嵁搴?- 浠婃棩瀹炴椂璁块棶閫氳繃 `k6` 璋冪湡瀹炵煭閾捐闂帴鍙?
涓叉垚涓€涓叆鍙ｏ紝閬垮厤浣犳墜宸ユ嫾涓€鍫嗗弬鏁般€?
---

## 涓€銆佸墠缃潯浠?
杩愯鍓嶈鍏堢‘璁わ細

1. 鍚庣鏈嶅姟宸茬粡鑳芥甯歌繛鎺?MySQL銆?2. 浣犱紶鍏ョ殑鐭爜閮界湡瀹炲瓨鍦ㄣ€?3. 濡傛灉瑕佹ā鎷熲€滀粖澶╃殑瀹炴椂璁块棶鈥濓紝鏈満宸插畨瑁?`k6`銆?4. 濡傛灉鍙兂琛ュ巻鍙叉暟鎹紝涓嶉渶瑕佸畨瑁?`k6`銆?
---

## 浜屻€佹渶甯哥敤鐨勪竴閿懡浠?
### 1. 鍘嗗彶鏁版嵁 + 浠婂ぉ瀹炴椂璁块棶锛屼竴璧风敓鎴?
```powershell
.\scripts\test\run_demo_seed.ps1 `
  -ShortCodes "52KD2a,nvGQWJQ0,lV86cWyd,Doubao,13dsw4" `
  -Preset demo-medium
```

杩欐潯鍛戒护浼氬仛涓や欢浜嬶細

1. 鐢?`go run ./cmd/seed-demo-data` 鍐欏叆鍘嗗彶璁块棶鏃ュ織
   - 涓昏瑕嗙洊鈥滀笂涓湀鈥濆拰鈥滄湰鏈?1~10 鍙封€?2. 鍐嶇敤 `k6` 妯℃嫙浠婂ぉ鐨勫疄鏃惰闂?
閫傚悎浣犺鐩存帴鍒锋柊鏁村婕旂ず鏁版嵁鏃朵娇鐢ㄣ€?
### 2. 鍙ˉ鍘嗗彶鏁版嵁

```powershell
.\scripts\test\run_demo_seed.ps1 `
  -ShortCodes "52KD2a,nvGQWJQ0,lV86cWyd,Doubao,13dsw4" `
  -Preset demo-medium `
  -WithLiveToday false
```

閫傚悎锛?
- 鍙兂琛?2 鏈堛€佹湰鏈堜笂鏃棫鏁版嵁
- 涓嶆兂椹笂鎵撲粖澶╃殑瀹炴椂娴侀噺
- 鏈満杩樻病瀹夎 `k6`

### 3. 鍙墦浠婂ぉ鐨勫疄鏃惰闂?
```powershell
.\scripts\test\run_demo_seed.ps1 `
  -ShortCodes "52KD2a,nvGQWJQ0,lV86cWyd,Doubao,13dsw4" `
  -Preset demo-medium `
  -WithHistory false
```

閫傚悎锛?
- 鍘嗗彶鏁版嵁宸茬粡琛ヨ繃
- 鍙兂璁╀粖澶╃殑瓒嬪娍銆佹潵婧愩€佺幆澧冨垎甯冨啀鍔ㄨ捣鏉?
### 4. 鎸囧畾鍚庣鍦板潃

```powershell
.\scripts\test\run_demo_seed.ps1 `
  -ShortCodes "52KD2a,nvGQWJQ0,lV86cWyd,Doubao,13dsw4" `
  -Preset demo-medium `
  -BaseUrl "http://localhost:8080"
```

---

## 涓夈€侀璁捐鏄?
褰撳墠鏀寔锛?
- `demo-small`
- `demo-medium`
- `demo-large`

寤鸿榛樿鐢細

```powershell
-Preset demo-medium
```

涓変釜棰勮鐨勫ぇ鑷村尯鍒細

1. `demo-small`
   - 鍘嗗彶鏁版嵁杈冨皯
   - 浠婃棩瀹炴椂娴侀噺杈冨皯
   - 閫傚悎鏈湴蹇€熻瘯璺?
2. `demo-medium`
   - 鏈€閫傚悎婕旂ず
   - 鑳藉舰鎴愭瘮杈冩槑鏄剧殑鎺掕銆佹潵婧愩€佺幆澧冨拰鍦板浘鍒嗗竷
   - 鎺ㄨ崘浼樺厛浣跨敤

3. `demo-large`
   - 鏁版嵁閲忔洿澶?   - 鏇村鏄撴妸澶撮儴鐭摼鎵撳埌涓婂崈鐐瑰嚮
   - 濡傛灉浣犳満鍣ㄨ祫婧愪竴鑸紝涓嶅缓璁绻侀噸澶嶈窇

---

## 鍥涖€佽剼鏈仛浜嗕粈涔?
### 1. 鍘嗗彶鏁版嵁鐩村啓鏁版嵁搴?
鍘嗗彶閫犳暟鍛戒护锛?
```powershell
go run .\cmd\seed-demo-data --short-codes "52KD2a,nvGQWJQ0,lV86cWyd"
```

瀹冧細锛?
1. 鏍￠獙鐭爜鏄惁瀛樺湪
2. 鎸夐璁剧敓鎴愨€滀笂涓湀 + 鏈湀涓婃棳鈥濈殑鍘嗗彶璁块棶鏃ュ織
3. 鑷姩鍐欏叆瀵瑰簲鏈堜唤鐨?`access_logs_YYYYMM`
4. 鍚屾澧為噺鏇存柊锛?   - `stats_daily`
   - `stats_region_daily`
   - `stats_device_daily`
   - `shortlinks.click_count`

娉ㄦ剰锛?
- 杩欐槸**澧為噺鍐欏叆**
- 澶氭杩愯浼氱户缁彔鍔犵偣鍑婚噺

### 2. 浠婃棩瀹炴椂璁块棶

瀹炴椂璁块棶鑴氭湰锛?
```powershell
k6 run .\scripts\test\seed_access_data.js
```

鍦ㄤ竴閿剼鏈噷瀹冧細鑷姩甯︿笂锛?
- 鍔犳潈鍚庣殑鐭爜鍒楄〃
- 鍔犳潈鍚庣殑鏉ユ簮姹?- 鍥藉唴澶栨贩鍚?IP 鏁伴噺涓庢瘮渚?- 骞跺彂鍜岃姹傛€婚噺

浣犱竴鑸笉闇€瑕佽嚜宸辨墜宸ュ啀鎷艰繖浜涘弬鏁般€?
---

## 浜斻€佹潵婧愭睜璇存槑

褰撳墠榛樿浼氭ā鎷熻繖浜涘父瑙佹潵婧愶細

### 鍥藉唴

- `weixin.qq.com`
- `weibo.com`
- `zhihu.com`
- `xiaohongshu.com`
- `douyin.com`
- `bilibili.com`
- `qq.com`
- `xiaoheihe.cn`
- `baidu.com`
- `tieba.baidu.com`

### 娴峰

- `google.com`
- `bing.com`
- `reddit.com`
- `x.com`
- `facebook.com`
- `instagram.com`
- `youtube.com`

鍙﹀杩樹細淇濈暀涓€閮ㄥ垎锛?
- `direct`锛堢洿鎺ヨ闂級

---

## 鍏€佹帹鑽愮煭鐮佷紶娉?
浣犵幇鍦ㄦ渶甯哥敤鐨勫啓娉曞氨鏄妸鈥滄垜鐨勯摼鎺モ€濋噷鍑嗗灞曠ず鐨勯偅鍑犳潯鐭爜鐩存帴浼犺繘鍘伙細

```powershell
.\scripts\test\run_demo_seed.ps1 `
  -ShortCodes "52KD2a,nvGQWJQ0,lV86cWyd,leNMP4LV,i652zhFD6xl,gIYc1lNwbGO,Doubao,2ZgkDzLhgOm,89sdss"
```

瑙勫垯璇存槑锛?
1. 鎸夐€楀彿鍒嗛殧
2. 涓嶈鎹㈣鎷嗗紑鐭爜鏈韩
3. 椤哄簭鏈夋剰涔?   - 鍓嶉潰鐨勭煭鐮佹潈閲嶆洿楂?   - 鏇村鏄撳舰鎴愬ご閮ㄧ儹闂ㄩ摼鎺?
涔熷氨鏄锛屽鏋滀綘鎯宠鏌?1~2 鏉￠摼鎺ユ洿鍍忊€滅垎娆锯€濓紝灏辨妸瀹冧滑鏀惧湪鏈€鍓嶉潰銆?
---

## 涓冦€侀€犳暟鍚庡浣曢獙璇?
寤鸿鎸夎繖涓『搴忕湅锛?
1. `鎴戠殑閾炬帴`
   - 鐪嬬偣鍑婚噺鏄惁寮€濮嬫媺寮€姊害
   - 鏄惁鍑虹幇灏戞暟楂樼偣鍑汇€佷竴浜涗腑绛夌偣鍑汇€佷竴浜涗綆鐐瑰嚮

2. `姒傝`
   - 鐪嬭秼鍔垮浘鏄惁鍙樺緱鏇村钩婊?   - 鐪嬭繍钀ユ彁閱掗噷鏄惁鏈夋洿鏄庢樉鐨勬暟鎹樊寮?
3. `鏁版嵁缁熻`
   - 鐪嬫潵婧愬垎甯冩槸鍚﹀嚭鐜?QQ銆佸皬榛戠洅銆佸井淇°€佺煡涔庣瓑
   - 鐪嬬幆澧冨垎甯冩槸鍚︽洿涓板瘜
   - 鐪嬩笘鐣屽湴鍥句笌涓浗鍦板浘鏄惁閮芥湁鍙睍绀虹偣浣?
4. `閾炬帴璇︽儏`
   - 鐪嬪崟閾炬帴瓒嬪娍銆佸湴鍩熴€佹潵婧愩€佹棩蹇楁槸鍚︽洿鍍忕湡瀹炰骇鍝佹暟鎹?
---

## 鍏€佸父瑙侀棶棰?
### 1. 鎶ラ敊 `鏈壘鍒?k6`

璇存槑浣犳湰鏈烘病瀹夎 `k6`銆?
瑙ｅ喅鍔炴硶锛?
1. 瀹夎 `k6`
2. 鎴栬€呭彧璺戝巻鍙叉暟鎹細

```powershell
.\scripts\test\run_demo_seed.ps1 `
  -ShortCodes "52KD2a,nvGQWJQ0,lV86cWyd" `
  -WithLiveToday false
```

### 2. 涓浗鍦板浘杩樻槸娌℃暟鎹?
鍏堢‘璁や袱浠朵簨锛?
1. 浣犲凡缁忚ˉ杩囧巻鍙叉暟鎹垨鎵撹繃浠婂ぉ鐨勭湡瀹炶闂?2. 杩欎簺璁块棶閲岀‘瀹炴湁涓浗鐪佷唤鏁版嵁

濡傛灉浣犲彧鏄墦浜嗗皯閲忔捣澶栬闂紝涓栫晫鍦板浘鍙兘姝ｅ父锛屼絾涓浗鍦板浘浠嶇劧寰堝急銆?
### 3. 澶氳窇鍑犳浼氫笉浼氶噸澶嶅彔鍔狅紵

浼氥€?
褰撳墠鑴氭湰璁捐灏辨槸澧為噺閫犳暟锛屼笉浼氬府浣犺嚜鍔ㄥ洖婊氭棫鏁版嵁銆?
濡傛灉浣犳兂閲嶆柊鏁寸悊涓€濂楁洿骞插噣鐨勬紨绀烘暟鎹紝寤鸿锛?
1. 鍏堝浠芥暟鎹簱
2. 鎴栧崟鐙噯澶囨紨绀虹幆澧?
### 4. 涓轰粈涔堝巻鍙叉暟鎹笉鐢?k6 鎵擄紵

鍥犱负鍘嗗彶鏁版嵁闇€瑕佷吉閫犺繃鍘荤殑鏃堕棿鐐广€?
璧扮湡瀹炶闂帴鍙ｅ彧鑳藉埗閫犫€滅幇鍦ㄥ彂鐢熺殑璁块棶鈥濓紝鍋氫笉鍑鸿嚜鐒剁殑 2 鏈堛€佹湰鏈堜笂鏃巻鍙叉洸绾匡紝鎵€浠ュ巻鍙查儴鍒嗛噰鐢ㄧ洿鍐欐暟鎹簱鏇村彲鎺с€?
---

## 涔濄€佷繚鐣欑殑鏃ц剼鏈?
鐩綍閲岃繖浜涜剼鏈繕淇濈暀鐫€锛屼絾鐜板湪閮戒笉鏄富鍏ュ彛锛?
- `st.js`
- `stip.js`
- `hst.js`
- `sl.bat`
- `s2.bat`

瀹冧滑涓昏鏄棭鏈熸祴璇曡剼鏈€?
褰撳墠鎺ㄨ崘浣犵粺涓€浣跨敤锛?
```powershell
.\scripts\test\run_demo_seed.ps1
```

