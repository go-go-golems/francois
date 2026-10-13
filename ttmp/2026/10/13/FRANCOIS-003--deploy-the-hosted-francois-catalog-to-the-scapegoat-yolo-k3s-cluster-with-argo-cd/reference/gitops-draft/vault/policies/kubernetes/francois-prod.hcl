# Serving identity: runtime contract and Pi provider files only.
path "kv/data/apps/francois/prod/runtime" {
  capabilities = ["read"]
}

path "kv/data/apps/francois/prod/pi" {
  capabilities = ["read"]
}
