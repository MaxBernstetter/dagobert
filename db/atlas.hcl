data "hcl_schema" "app" {
  paths = fileset("schema/**/*.pg.hcl")
}

env "local" {
  url = getenv("DATABASE_URL")
  # Atlas OSS: stock docker:// image (Pro `docker` block requires atlas login).
  dev = "docker://postgres/16/dev?search_path=public"
  src = data.hcl_schema.app.url
  schemas = ["public"]
  migration {
    dir              = "file://migrations"
    revisions_schema = "atlas"
  }
}
