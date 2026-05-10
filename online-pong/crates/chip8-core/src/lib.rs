//! Pure CHIP-8 emulator core.
//!
//! Intentionally I/O-free: no file system, no network, no audio, no wall-clock
//! time. Given the same inputs it always produces the same outputs, making
//! deterministic snapshot-based rollback tractable in later epics.

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
