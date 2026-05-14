package domain

type InvoiceProfileSnapshot struct {
	Title       string  `json:"title"`
	TaxNumber   string  `json:"tax_number"`
	Email       string  `json:"email"`
	Address     *string `json:"address,omitempty"`
	Phone       *string `json:"phone,omitempty"`
	BankName    *string `json:"bank_name,omitempty"`
	BankAccount *string `json:"bank_account,omitempty"`
}
