env "local" {
  src = "file://migrations"
  url = "postgres://shortener:shortener@localhost:5432/shortener?sslmode=disable"
  dev = "docker://postgres/17/dev?search_path=public"

  migration {
    dir = "file://migrations"
  }
}
