# Bootstrap Job: cluster admin credentials plus the app's database contract.
path "kv/data/infra/postgres/cluster" {
  capabilities = ["read"]
}

path "kv/data/apps/francois/prod/runtime" {
  capabilities = ["read"]
}
