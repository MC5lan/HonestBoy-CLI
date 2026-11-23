package core

// Authority 是权限分级（文档 §3.4.2）。
type Authority int

const (
	AuthReadOnly     Authority = iota // 只读：无需确认
	AuthWrite                         // 写文件：默认允许，可配置确认
	AuthIrreversible                  // 删除/覆盖/外发：必须显式授权，单次有效
)

// Authorize 判定动作是否被允许。
// AuthIrreversible 的授权单次有效，不设“永久允许”（文档 F5）。
func Authorize(a Authority, granted map[Authority]bool) bool {
	switch a {
	case AuthReadOnly:
		return true
	case AuthWrite:
		return granted[AuthWrite]
	case AuthIrreversible:
		return granted[AuthIrreversible]
	default:
		return false
	}
}
