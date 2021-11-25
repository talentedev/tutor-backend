package models

type ChargeData struct {
	TutorPay    int64   `json:"tutor_pay,omitempty" bson:"tutor_pay,omitempty"`
	TutorRate   float32 `json:"tutor_rate,omitempty" bson:"tutor_rate,omitempty"`
	PlatformFee int64   `json:"platform_fee,omitempty" bson:"platform_fee,omitempty"`
	StudentCost int64   `json:"student_cost,omitempty" bson:"student_cost,omitempty"`
	ChargeID    string  `json:"charge_id,omitempty" bson:"charge_id,omitempty"`
}

