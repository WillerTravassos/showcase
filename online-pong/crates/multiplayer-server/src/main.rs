//! Multi-game multiplayer server.
//!
//! A `tokio` + `axum` HTTP service handling matchmaking, session orchestration,
//! and score persistence via `sqlx` with `PostgreSQL`. Designed to run as a
//! Kubernetes Deployment; see `deploy/` for manifests.

fn main() {
    println!("Hello, world!");
}
