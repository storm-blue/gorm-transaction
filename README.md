## GROM TransactionManager

```
db := ...
tm := NewTransactionManager(db)

repositoryA := &RepositoryA{
    BaseRepository: NewBaseRepository(db)
}

repositoryB := &RepositoryB{
    BaseRepository: NewBaseRepository(db)
}

repositoryB := &RepositoryC{
    BaseRepository: NewBaseRepository(db)
}

err := tm.Transaction(
	ctx,
	func(ctx context.Context, tx *gorm.DB) error {
		tx.Create(...)
		repository1.Create(ctx, ...)
		repository2.Create(ctx, ...)
		repository3.Create(ctx, ...)
		
		// any error return, transaction will rollback
		return nil
	},
	PropagationSupports,
)

println(err.Error())
```