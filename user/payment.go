package user

type PaymentMethod int

//goland:noinspection ALL
const (
	PaymentMethodInvalid PaymentMethod = iota
	PaymentMethodBit
	PaymentMethodPaybox
	PaymentMethodPepper
)

var paymentsString = map[PaymentMethod]string{
	PaymentMethodBit:    "Bit",
	PaymentMethodPaybox: "Paybox",
	PaymentMethodPepper: "Pepper pay",
}

func (p PaymentMethod) String() string {
	return paymentsString[p]
}

type Payment struct {
}
