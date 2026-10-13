# The francois source workflow can read only the GitHub App credential used to
# propose an image pin in the k3s GitOps repository.
path "kv/data/ci/github/francois/gitops-pr-app" {
  capabilities = ["read"]
}

path "auth/token/lookup-self" {
  capabilities = ["read"]
}

path "auth/token/renew-self" {
  capabilities = ["update"]
}

path "auth/token/revoke-self" {
  capabilities = ["update"]
}
