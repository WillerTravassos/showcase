//! Rollback netplay integration for CHIP-8.
//!
//! Wraps `ggrs` behind a stable API so the desktop frontend does not need to
//! change when the rollback implementation is replaced in Epic 11. This crate
//! is fully synchronous — no `async fn` lives here.

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
