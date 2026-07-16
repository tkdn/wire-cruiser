package structapp

type DB struct{}

type Config struct {
	DB *DB
}

func LoadConfig() *Config {
	return &Config{DB: &DB{}}
}

type App struct {
	DB *DB
}
