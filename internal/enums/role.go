package enums

type Role int

const (
	RoleUser Role = iota
	RoleAdmin
	RoleUnknown
)

func (r Role) String() string {
	switch r {
	case RoleUser:
		return "user"
	case RoleAdmin:
		return "admin"
	default:
		return "unknown"
	}
}

func ParseRole(value string) Role {
	switch value {
	case "user":
		return RoleUser
	case "admin":
		return RoleAdmin
	default:
		return RoleUnknown
	}
}
