package options

// TCP/UDP port range for endpoints (IANA user port / dynamic range).
// 服务端口的合法 TCP/UDP 取值范围。
const (
	minPort = 1
	maxPort = 65535
)