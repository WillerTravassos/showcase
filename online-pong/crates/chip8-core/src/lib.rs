//! Pure CHIP-8 emulator core.
//!
//! Intentionally I/O-free: no file system, no network, no audio, no wall-clock
//! time. Given the same inputs it always produces the same outputs, making
//! deterministic snapshot-based rollback tractable.

/// Height of the CHIP-8 display in pixels.
pub const DISPLAY_HEIGHT: usize = 32;

/// Width of the CHIP-8 display in pixels.
pub const DISPLAY_WIDTH: usize = 64;

/// The interpreter's reserved area also stores built-in font sprites. Convention puts them at 0x50. Opcode FX29 returns
/// the address of the sprite for hex digit X — it can only work if you put the font there at startup
const FONTSET_START_ADDRESS: usize = 0x50;

const FONTSET_SIZE: usize = 80;

const FONTSET: [u8; FONTSET_SIZE] = [
    0xF0, 0x90, 0x90, 0x90, 0xF0, // 0
    0x20, 0x60, 0x20, 0x20, 0x70, // 1
    0xF0, 0x10, 0xF0, 0x80, 0xF0, // 2
    0xF0, 0x10, 0xF0, 0x10, 0xF0, // 3
    0x90, 0x90, 0xF0, 0x10, 0x10, // 4
    0xF0, 0x80, 0xF0, 0x10, 0xF0, // 5
    0xF0, 0x80, 0xF0, 0x90, 0xF0, // 6
    0xF0, 0x10, 0x20, 0x40, 0x40, // 7
    0xF0, 0x90, 0xF0, 0x90, 0xF0, // 8
    0xF0, 0x90, 0xF0, 0x10, 0xF0, // 9
    0xF0, 0x90, 0xF0, 0x90, 0x90, // A
    0xE0, 0x90, 0xE0, 0x90, 0xE0, // B
    0xF0, 0x80, 0x80, 0x80, 0xF0, // C
    0xE0, 0x90, 0x90, 0x90, 0xE0, // D
    0xF0, 0x80, 0xF0, 0x80, 0xF0, // E
    0xF0, 0x80, 0xF0, 0x80, 0x80, // F
];

/// Total addressable memory in bytes.
pub const MEMORY_SIZE: usize = 4096;

/// Address where ROM programs are loaded. Bytes `0x000–0x1FF` are reserved for the interpreter.
/// Programs always start on the address defined here.
pub const START_ADDRESS: u16 = 0x200;

/// Models the hardware state of the CHIP-8 virtual machine.
///
/// Every field maps to a physical component of the original COSMAC VIP.
#[allow(dead_code)] // fields written in new(); readers added as opcodes are implemented (Epic 2)
#[derive(Debug)]
pub struct Chip8 {
    /// 4KB memory. Hardware limit enforced at compile time
    memory: [u8; MEMORY_SIZE],
    /// 16 general purpose 8bit registers (V0 to VF). VF a special byte that acts as flag to check if
    /// carry, borrow, or collision
    registers: [u8; 16],
    /// Index registers hold memory addresses. Addresses are 12-bit
    index_register: u16,
    /// Program counter points to the current emulator instruction
    program_counter: u16,
    /// Hardware call stack for subroutines. It stores the 16-bit return addresses
    stack: [u16; 16],
    /// Index into stack
    stack_pointer: u8,
    /// Counts down at 60Hz, general-purpose timing
    delay_timer: u8,
    /// Counts down at 60Hz, same as delay timer, but for sounds
    sound_timer: u8,
    /// 64 x 32 monochrome screen. Bool per pixel
    display: [bool; DISPLAY_WIDTH * DISPLAY_HEIGHT],
    /// Hex keypad
    keypad: [bool; 16],
}

impl Chip8 {
    /// Initiates a new CHIP-8 emulator core
    pub fn new() -> Self {
        let mut memory = [0u8; 4096];

        // NOTE: This effectively replaces the for loop bellow:
        // for i in 0..FONTSET_SIZE {
        //     memory[FONTSET_START_ADDRESS + i] = FONTSET[i];
        // }

        memory[FONTSET_START_ADDRESS..(FONTSET_SIZE + FONTSET_START_ADDRESS)].copy_from_slice(&FONTSET);

        Self {
            memory,
            registers: [0u8; 16],
            index_register: 0u16,
            program_counter: START_ADDRESS,
            stack: [0u16; 16],
            stack_pointer: 0u8,
            delay_timer: 0u8,
            sound_timer: 0u8,
            display: [false; DISPLAY_WIDTH * DISPLAY_HEIGHT],
            keypad: [false; 16],
        }
    }
}

impl Default for Chip8 {
    fn default() -> Self {
        Self::new()
    }
}

#[cfg(test)]
mod chip_core_tests {
    use pretty_assertions::assert_eq;

    use crate::{Chip8, FONTSET, FONTSET_SIZE, FONTSET_START_ADDRESS, START_ADDRESS};

    #[test]
    fn new() {
        let fontset_start_address = FONTSET_START_ADDRESS;
        let fontset_end_address = FONTSET_START_ADDRESS + FONTSET_SIZE;
        let chip8_emulator = Chip8::new();

        assert_eq!(chip8_emulator.program_counter, START_ADDRESS);
        assert_eq!(&chip8_emulator.memory[fontset_start_address..fontset_end_address], &FONTSET);
    }
}
