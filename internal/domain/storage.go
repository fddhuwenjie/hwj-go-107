package domain

// 存储单元类型。
const (
	UnitWarehouse = "warehouse" // 库房
	UnitShowcase  = "showcase"  // 展柜
)

// 存储单元状态。
const (
	UnitActive   = "active"
	UnitDisabled = "disabled"
)

// StorageUnit 存储单元（库房或展柜）。
type StorageUnit struct {
	ID        int64  `json:"id"`
	Code      string `json:"code"`
	Name      string `json:"name"`
	Kind      string `json:"kind"`
	Location  string `json:"location"`
	Status    string `json:"status"`
	Version   int64  `json:"version"`
	CreatedAt int64  `json:"created_at"`
	UpdatedAt int64  `json:"updated_at"`
}

// Sensor 温湿度传感器。
type Sensor struct {
	ID            int64  `json:"id"`
	Code          string `json:"code"`
	StorageUnitID int64  `json:"storage_unit_id"`
	Kind          string `json:"kind"`
	Status        string `json:"status"`
	CreatedAt     int64  `json:"created_at"`
}
