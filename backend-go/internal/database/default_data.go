package database

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

const DefaultSRRConfig = `{"version":"1","rules":[{"name":"Browser Subscription","description":"System critical: do not delete or disable this rule.","enabled":true,"operator":"AND","conditions":[{"headerName":"accept","operator":"CONTAINS","value":"text/html","caseSensitive":true}],"responseType":"BROWSER"},{"name":"Mihomo Clients","description":"Response with generated YAML config (Mihomo Template)","enabled":true,"operator":"AND","conditions":[{"headerName":"user-agent","operator":"REGEX","value":"^(?:flclash|rabbit|flowvy|murge|mihomo|prizrak-box|koala-clash|clash(?:-verge|-nyanpasu|x meta|[-.]?meta))","caseSensitive":false}],"responseType":"MIHOMO"},{"name":"Stash (iOS, macOS)","description":"Response with generated YAML config (Stash Template)","enabled":true,"operator":"AND","conditions":[{"headerName":"user-agent","operator":"REGEX","value":"^stash","caseSensitive":false}],"responseType":"STASH"},{"name":"Sing-box clients","description":"Response with generated JSON config (Singbox template)","enabled":true,"operator":"AND","conditions":[{"headerName":"user-agent","operator":"REGEX","value":"^sfa|sfi|sfm|sft|karing|singbox","caseSensitive":false}],"responseType":"SINGBOX"},{"name":"Clash Core Clients","description":"Response with generated YAML config (Clash Template)","enabled":true,"operator":"AND","conditions":[{"headerName":"user-agent","operator":"REGEX","value":"^clash","caseSensitive":false}],"responseType":"CLASH"},{"name":"Fallback Base64","description":"System critical: do not delete or disable this rule.","enabled":true,"operator":"AND","conditions":[],"responseType":"XRAY_BASE64"}]}`

const DefaultTemplateMihomo = `mixed-port: 7890
socks-port: 7891
redir-port: 7892
allow-lan: true
mode: global
log-level: info
external-controller: 127.0.0.1:9090
dns:
  enable: true
  use-hosts: true
  enhanced-mode: fake-ip
  fake-ip-range: 198.18.0.1/16
  default-nameserver:
    - 1.1.1.1
    - 8.8.8.8
  nameserver:
    - 1.1.1.1
    - 8.8.8.8
  fake-ip-filter:
    - '*.lan'
    - stun.*.*.*
    - stun.*.*
    - time.windows.com
    - time.nist.gov
    - time.apple.com
    - time.asia.apple.com
    - '*.openwrt.pool.ntp.org'
    - pool.ntp.org
    - ntp.ubuntu.com
    - time1.apple.com
    - time2.apple.com
    - time3.apple.com
    - time4.apple.com
    - time5.apple.com
    - time6.apple.com
    - time7.apple.com
    - time1.google.com
    - time2.google.com
    - time3.google.com
    - time4.google.com
    - api.joox.com
    - joox.com
    - '*.xiami.com'
    - '*.msftconnecttest.com'
    - '*.msftncsi.com'
    - '+.xboxlive.com'
    - '*.*.stun.playstation.net'
    - xbox.*.*.microsoft.com
    - '*.ipv6.microsoft.com'
    - speedtest.cros.wr.pvp.net

proxies:

proxy-groups:
  - name: '→ GoWave'
    type: 'select'
    proxies:

rules:
  - MATCH,→ GoWave
`

const DefaultTemplateStash = `proxy-groups:
  - name: → GoWave
    type: select
    proxies:

proxies:

rules:
  - SCRIPT,quic,REJECT
  - DOMAIN-SUFFIX,iphone-ld.apple.com,DIRECT
  - DOMAIN-SUFFIX,lcdn-locator.apple.com,DIRECT
  - DOMAIN-SUFFIX,lcdn-registration.apple.com,DIRECT
  - DOMAIN-SUFFIX,push.apple.com,DIRECT
  - PROCESS-NAME,v2ray,DIRECT
  - PROCESS-NAME,Surge,DIRECT
  - PROCESS-NAME,ss-local,DIRECT
  - PROCESS-NAME,privoxy,DIRECT
  - PROCESS-NAME,trojan,DIRECT
  - PROCESS-NAME,trojan-go,DIRECT
  - PROCESS-NAME,naive,DIRECT
  - PROCESS-NAME,CloudflareWARP,DIRECT
  - PROCESS-NAME,Cloudflare WARP,DIRECT
  - IP-CIDR,162.159.193.0/24,DIRECT,no-resolve
  - PROCESS-NAME,p4pclient,DIRECT
  - PROCESS-NAME,Thunder,DIRECT
  - PROCESS-NAME,DownloadService,DIRECT
  - PROCESS-NAME,qbittorrent,DIRECT
  - PROCESS-NAME,Transmission,DIRECT
  - PROCESS-NAME,fdm,DIRECT
  - PROCESS-NAME,aria2c,DIRECT
  - PROCESS-NAME,Folx,DIRECT
  - PROCESS-NAME,NetTransport,DIRECT
  - PROCESS-NAME,uTorrent,DIRECT
  - PROCESS-NAME,WebTorrent,DIRECT
  - GEOIP,LAN,DIRECT
  - MATCH,→ GoWave
script:
  shortcuts:
    quic: network == 'udp' and dst_port == 443
dns:
  default-nameserver:
    - 1.1.1.1
    - 1.0.0.1
  nameserver:
    - 1.1.1.1
    - 1.0.0.1
log-level: warning
mode: rule
`

const DefaultTemplateClash = `mixed-port: 7890
socks-port: 7891
redir-port: 7892
allow-lan: true
mode: global
log-level: info
external-controller: 127.0.0.1:9090
dns:
  enable: true
  use-hosts: true
  enhanced-mode: fake-ip
  fake-ip-range: 198.18.0.1/16
  default-nameserver:
    - 1.1.1.1
    - 8.8.8.8
  nameserver:
    - 1.1.1.1
    - 8.8.8.8
  fake-ip-filter:
    - '*.lan'
    - stun.*.*.*
    - stun.*.*
    - time.windows.com
    - time.nist.gov
    - time.apple.com
    - time.asia.apple.com
    - '*.openwrt.pool.ntp.org'
    - pool.ntp.org
    - ntp.ubuntu.com
    - time1.apple.com
    - time2.apple.com
    - time3.apple.com
    - time4.apple.com
    - time5.apple.com
    - time6.apple.com
    - time7.apple.com
    - time1.google.com
    - time2.google.com
    - time3.google.com
    - time4.google.com
    - api.joox.com
    - joox.com
    - '*.xiami.com'
    - '*.msftconnecttest.com'
    - '*.msftncsi.com'
    - '+.xboxlive.com'
    - '*.*.stun.playstation.net'
    - xbox.*.*.microsoft.com
    - '*.ipv6.microsoft.com'
    - speedtest.cros.wr.pvp.net

proxies:

proxy-groups:
  - name: '→ GoWave'
    type: 'select'
    proxies:

rules:
  - MATCH,→ GoWave
`

const DefaultTemplateSingbox = `{"dns":{"rules":[{"server":"remote","query_type":["A","AAAA"]}],"servers":[{"tag":"cf-dns","type":"tls","server":"1.1.1.1"},{"tag":"local","type":"udp","server":"1.1.1.1"},{"tag":"remote","type":"fakeip","inet4_range":"198.18.0.0/15","inet6_range":"fc00::/18"}],"independent_cache":true},"log":{"level":"debug","disabled":true,"timestamp":true},"route":{"rules":[{"action":"sniff"},{"action":"hijack-dns","protocol":"dns"},{"outbound":"direct","ip_is_private":true}],"auto_detect_interface":true,"default_domain_resolver":"local"},"inbounds":[{"mtu":9000,"tag":"tun-in","type":"tun","stack":"mixed","address":["172.19.0.1/30","fdfe:dcba:9876::1/126"],"platform":{"http_proxy":{"server":"127.0.0.1","enabled":true,"server_port":2412}},"auto_route":true,"strict_route":true,"interface_name":"tun125","endpoint_independent_nat":true},{"tag":"mixed-in","type":"mixed","users":[],"listen":"127.0.0.1","listen_port":2412,"set_system_proxy":false}],"outbounds":[{"tag":"→ GoWave","type":"selector","outbounds":null,"interrupt_exist_connections":true},{"tag":"direct","type":"direct"}],"experimental":{"clash_api":{"external_ui":"yacd","default_mode":"rule","external_controller":"127.0.0.1:9090","external_ui_download_url":"https://github.com/MetaCubeX/Yacd-meta/archive/gh-pages.zip","external_ui_download_detour":"direct"},"cache_file":{"path":"remnawave.db","enabled":true,"cache_id":"remnawave","store_fakeip":true}}}`

const DefaultTemplateXrayJson = `{"dns":{"servers":["1.1.1.1","1.0.0.1"],"queryStrategy":"UseIP"},"routing":{"rules":[{"type":"field","protocol":["bittorrent"],"outboundTag":"direct"}],"domainMatcher":"hybrid","domainStrategy":"IPIfNonMatch"},"inbounds":[{"tag":"socks","port":10808,"listen":"127.0.0.1","protocol":"socks","settings":{"udp":true,"auth":"noauth"},"sniffing":{"enabled":true,"routeOnly":false,"destOverride":["http","tls","quic"]}},{"tag":"http","port":10809,"listen":"127.0.0.1","protocol":"http","settings":{"allowTransparent":false},"sniffing":{"enabled":true,"routeOnly":false,"destOverride":["http","tls","quic"]}}],"outbounds":[{"tag":"direct","protocol":"freedom"},{"tag":"block","protocol":"blackhole"}]}`

const DefaultSubpageConfig = `{"version":"1","locales":["en","ru","zh","fa","fr"],"brandingSettings":{"title":"Subscription","logoUrl":"https://docs.rw/img/logo.svg","supportUrl":"https://dummy.docs.rw"},"uiConfig":{"subscriptionInfoBlockType":"expanded","installationGuidesBlockType":"cards"},"baseSettings":{"metaTitle":"Subscription","metaDescription":"Subscription","showConnectionKeys":false,"hideGetLinkButton":false},"baseTranslations":{"installationGuideHeader":{"en":"Installation","ru":"Установка","fa":"نصب","fr":"Installation","zh":"安装"},"connectionKeysHeader":{"en":"Connection Keys","ru":"Ключи подключения","fa":"کلیدهای اتصال","fr":"Clés de connexion","zh":"连接密钥"},"linkCopied":{"en":"Link copied","ru":"Ссылка скопирована","fr":"Lien copié","fa":"لینک کپی شد","zh":"链接已复制"},"linkCopiedToClipboard":{"en":"Link copied to clipboard","ru":"Ссылка скопирована в буфер обмена","fr":"Lien copié dans le presse-papiers","fa":"لینک به کلیپ\u200cبورد کپی شد","zh":"链接已复制到剪贴板"},"getLink":{"en":"Get Link","ru":"Получение ссылки","fr":"Obtenir le lien","fa":"دریافت لینک","zh":"获取链接"},"scanQrCode":{"en":"Scan the QR code above in the client","ru":"Отсканируйте QR-код в приложении","fr":"Scannez le code QR dans l'application","fa":"کد QR بالا را در کلاینت اسکن کنید","zh":"在客户端中扫描上方二维码"},"scanQrCodeDescription":{"en":"Easily add the subscription to any client. There's another option: copy the link below and paste it into the client","ru":"Простое добавление подписки в любой клиент. Есть и другой вариант: скопируйте ссылку ниже и вставьте в клиент.","fr":"Ajoutez facilement l'abonnement à n'importe quel client. Vous pouvez aussi copier le lien ci-dessous et le coller dans le client.","fa":"افزودن آسان اشتراک به هر کلاینت. گزینه دیگری هم وجود دارد: لینک زیر را کپی کرده و در کلاینت جای\u200cگذاری کنید","zh":"轻松将订阅添加到任何客户端。还有另一种选择：复制下面的链接并粘贴到客户端中"},"copyLink":{"en":"Copy link","ru":"Скопировать ссылку","fr":"Copier le lien","fa":"کپی لینک","zh":"复制链接"},"name":{"en":"Username","ru":"Имя пользователя","fr":"Nom d'utilisateur","fa":"نام کاربری","zh":"用户名"},"status":{"en":"Status","ru":"Статус","fr":"Statut","fa":"وضعیت","zh":"状态"},"active":{"en":"Active","ru":"Активна","fr":"Active","fa":"فعال","zh":"活跃"},"inactive":{"en":"Inactive","ru":"Неактивна","fr":"Inactive","fa":"غیرفعال","zh":"未激活"},"expires":{"en":"Expires","ru":"Истекает","fr":"Expire","fa":"منقضی می\u200cشود","zh":"到期时间"},"at":{"en":"At","ru":"В","fr":"À","fa":"در","zh":"于"},"bandwidth":{"en":"Bandwidth","ru":"Трафик","fr":"Bande passante","fa":"پهنای باند","zh":"流量"},"scanToImport":{"en":"Scan to import this key","ru":"Отсканируйте QR-код для импорта ключа","fr":"Scannez le code QR pour importer cette clé","fa":"برای وارد کردن این کلید اسکن کنید","zh":"扫描以导入此密钥"},"expiresIn":{"en":"Expires","ru":"Истекает","fr":"Expire","fa":"منقضی می\u200cشود در","zh":"后到期"},"expired":{"en":"Expired","ru":"Истекла","fr":"Expirée","fa":"منقضی شده در","zh":"已于"},"unknown":{"en":"Unknown","ru":"Неизвестно","fr":"Inconnu","fa":"نامعلوم","zh":"未知"},"indefinitely":{"en":"Indefinitely","ru":"Бессрочно","fr":"Indéfiniment","fa":"هیچوقت","zh":"永久"}}}`

const DefaultCustomRemarks = `{"expiredUsers":["Subscription Expired"],"limitedUsers":["Traffic Limit Reached"],"disabledUsers":["Subscription Disabled"],"emptyHosts":["No Hosts Available"],"HWIDMaxDevicesExceeded":["Device Limit Exceeded"],"HWIDNotSupported":["HWID Not Supported"]}`

func SeedDefaults(db *gorm.DB) {
	var settingsCount int64
	db.Model(&PanelSetting{}).Count(&settingsCount)
	if settingsCount == 0 {
		db.Create(&PanelSetting{
			Title:               "GoWave",
			IsLoginAllowed:      true,
			IsRegisterAllowed:   false,
			PasswordAuthEnabled: true,
		})
	}

	var subSettingCount int64
	db.Model(&SubscriptionSetting{}).Count(&subSettingCount)
	if subSettingCount == 0 {
		now := time.Now().UTC()
		db.Create(&SubscriptionSetting{
			UUID:                        "00000000-0000-0000-0000-000000000000",
			ServeJsonAtBaseSubscription: false,
			IsShowCustomRemarks:         true,
			CustomRemarks:               DefaultCustomRemarks,
			CustomResponseHeaders:       "{}",
			RandomizeHosts:              false,
			ResponseRules:               DefaultSRRConfig,
			HwidSettings:                `{"enabled":false,"fallbackDeviceLimit":0,"maxDevicesAnnounce":null}`,
			CreatedAt:                   now,
			UpdatedAt:                   now,
		})
	}

	var subPageCount int64
	db.Model(&SubscriptionPageConfig{}).Count(&subPageCount)
	if subPageCount == 0 {
		now := time.Now().UTC()
		db.Create(&SubscriptionPageConfig{
			UUID:         "00000000-0000-0000-0000-000000000000",
			ViewPosition: 1,
			Name:         "Default",
			Tags:         StringArray{},
			Config:       DefaultSubpageConfigData,
			CreatedAt:    now,
			UpdatedAt:    now,
		})
	}

	defaults := []struct {
		Type string
		Yaml string
		Json string
		Pos  int
	}{
		{Type: "MIHOMO", Yaml: DefaultTemplateMihomo, Json: "", Pos: 1},
		{Type: "CLASH", Yaml: DefaultTemplateClash, Json: "", Pos: 2},
		{Type: "STASH", Yaml: DefaultTemplateStash, Json: "", Pos: 3},
		{Type: "SINGBOX", Yaml: "", Json: DefaultTemplateSingbox, Pos: 4},
		{Type: "XRAY_JSON", Yaml: "", Json: DefaultTemplateXrayJson, Pos: 5},
	}

	for _, item := range defaults {
		var cnt int64
		db.Model(&SubscriptionTemplate{}).Where("template_type = ?", item.Type).Count(&cnt)
		if cnt == 0 {
			now := time.Now().UTC()
			db.Create(&SubscriptionTemplate{
				UUID:         uuid.New().String(),
				ViewPosition: item.Pos,
				Name:         "Default",
				Tags:         StringArray{},
				TemplateType: item.Type,
				TemplateYaml: item.Yaml,
				TemplateJson: item.Json,
				CreatedAt:    now,
				UpdatedAt:    now,
			})
		}
	}
}

func EnsureSubscriptionSetting(db *gorm.DB) *SubscriptionSetting {
	var s SubscriptionSetting
	if err := db.First(&s).Error; err != nil {
		now := time.Now().UTC()
		s = SubscriptionSetting{
			UUID:                        "00000000-0000-0000-0000-000000000000",
			ServeJsonAtBaseSubscription: false,
			IsShowCustomRemarks:         true,
			CustomRemarks:               DefaultCustomRemarks,
			CustomResponseHeaders:       "{}",
			RandomizeHosts:              false,
			ResponseRules:               DefaultSRRConfig,
			HwidSettings:                `{"enabled":false,"fallbackDeviceLimit":0,"maxDevicesAnnounce":null}`,
			CreatedAt:                   now,
			UpdatedAt:                   now,
		}
		db.Create(&s)
	}
	return &s
}
