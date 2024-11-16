package transaction

import "gorm.io/gorm"

type BaseRepository interface {
	TransactionManager
}

func NewBaseRepository(db *gorm.DB) BaseRepository {
	return NewTransactionManager(db)
}
