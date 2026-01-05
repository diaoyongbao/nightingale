package dbm

// ArcheryConfig Archery 集成配置
type ArcheryConfig struct {
	Enable             bool   `toml:"enable" json:"enable"`
	Address            string `toml:"address" json:"address"`
	AuthType           string `toml:"auth_type" json:"auth_type"` // token, basic, session
	AuthToken          string `toml:"auth_token" json:"auth_token"`
	Username           string `toml:"username" json:"username"`
	Password           string `toml:"password" json:"password"`
	Timeout            int    `toml:"timeout" json:"timeout"`                         // 超时时间(毫秒)
	ConnectTimeout     int    `toml:"connect_timeout" json:"connect_timeout"`         // 连接超时(毫秒)
	InsecureSkipVerify bool   `toml:"insecure_skip_verify" json:"insecure_skip_verify"` // 是否跳过 SSL 验证
}
