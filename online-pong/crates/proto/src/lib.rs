//! Shared wire types for client–server communication.
//!
//! Pure data-transfer objects with `serde` derives. No business logic lives
//! here. Both `chip8-frontend` and `multiplayer-server` depend on this crate.

fn add(left: u64, right: u64) -> u64 {
    left + right
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn it_works() {
        let result = add(2, 2);
        assert_eq!(result, 4);
    }
}
