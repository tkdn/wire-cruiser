package missingapp

type DB struct{}

type App struct {
	DB *DB
}

func NewApp(db *DB) *App {
	return &App{DB: db}
}
