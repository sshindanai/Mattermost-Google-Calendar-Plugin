package dbmodel

type ConnectStates struct {
	UserID string `json:"user_id"`
	State  string `json:"state"`
}

func (*ConnectStates) TableName() string {
	return "connect_states"
}
