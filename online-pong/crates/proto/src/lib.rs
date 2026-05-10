//! Shared wire types for client–server communication.
//!
//! Pure data-transfer objects with `serde` derives. No business logic lives
//! here. Both `chip8-frontend` and `multiplayer-server` depend on this crate.

#[cfg(test)]
mod tests {}
