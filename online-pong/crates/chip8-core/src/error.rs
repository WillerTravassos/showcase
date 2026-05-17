/// Result of CHIP-8 core function calls
pub type Result<T> = std::result::Result<T, Error>;

/// Defines error returned by the CHIP-8 emulator when an error arises from a running programs.
#[derive(Debug, thiserror::Error, PartialEq, Eq)]
pub enum Error {
    /// Program tried to push an address onto the stack, but the stack was full.
    /// This arises when, for example, a Call Subroutine instruction (2NNN) is executed, but the
    /// stack pointer (Chip8.sp) is already at its limit (15).
    #[error("stack overflow")]
    StackOverflow,

    /// Program tried to pop an address off the stack, but the stack was already empty.
    /// This can arise when, for example, a Return From Subroutine instruction (00EE), but the stack
    /// pointer is at 0 (no address to ready).
    #[error("stack underflow")]
    StackUnderflow,

    /// CPU core tried to fetch a 2-byte instruction from memory that does not map to any valid
    /// CHIP-8 command.
    /// This arises when, for example, the program counter (Chip8.pc) points to memory that has
    /// data (sprites, sound, etc) instead of executable code, or the ROM is corrupted.
    #[error("invalid opcode: {0:#06X}")]
    InvalidOpcode(u16),

    /// Program tried to read from/write to/jump to a memory location outside the allowed boundary.
    /// This arises when the index register (Chip8.i) or the program counter (Chip8.pc) target an
    /// address greater than 0xFFFl. CHIP-8 has a 4096 bytes of RAM with addresses ranging from
    /// 0x000 to 0xFFF.
    #[error("invalid address: {0:#06X}")] // hex value string format, pads with leading zeroes for a
    // max length of 6.
    InvalidAddress(u16),

    /// ROM file (aka, a game), exceeds the available system memory.
    /// This arises when a rom size exceeds the 3584 bytes available for ROMs. CHIP-8 reserves the
    /// first 512 bytes (0x000 to 0x1FF) for the fontset (aka, system interpreter). Programs start
    /// at address 0x200.
    #[error("ROM too large: {size} bytes")]
    RomTooLarge {
        /// Size of the ROM in bytes.
        size: usize,
    },
}
