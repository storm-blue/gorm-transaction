package transaction

import "gorm.io/gorm"

type Repository interface {
	TransactionManager
}

func NewRepository(db *gorm.DB) Repository {
	return NewTransactionManager(db)
}
