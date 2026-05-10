//! Rollback netplay integration for CHIP-8.
//!
//! Wraps `ggrs` behind a stable API so the desktop frontend does not need to
//! change when the rollback implementation is replaced in Epic 11. This crate
//! is fully synchronous — no `async fn` lives here.

#[cfg(test)]
mod tests {}
