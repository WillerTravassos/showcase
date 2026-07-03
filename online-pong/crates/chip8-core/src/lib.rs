#![deny(unsafe_code)]
#![warn(missing_docs)]
#![warn(rust_2018_idioms)]
//! Pure CHIP-8 emulator core.
//!
//! Intentionally I/O-free: no file system, no network, no audio, no wall-clock
//! time. Given the same inputs it always produces the same outputs, making
//! deterministic snapshot-based rollback tractable.

/// Errors module containing all errors that may be returned during the run of a CHIP-8 ROM
pub mod error;

mod cpu;
mod display;
mod keypad;
mod memory;
mod opcodes;
mod timers;

pub use error::{Error, Result};

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

/// Maximum allowed size for CHIP-8 ROMs is 3584 bytes.
pub const MAX_ROM_SIZE: usize = MEMORY_SIZE - (START_ADDRESS as usize);

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
    v: [u8; 16],
    /// Index registers hold memory addresses. Addresses are 12-bit
    i: u16,
    /// Program counter points to the current emulator instruction
    pc: u16,
    /// Hardware call stack for subroutines. It stores the 16-bit return addresses
    stack: [u16; 16],
    /// Index into stack
    sp: u8,
    /// Counts down at 60Hz, general-purpose timing
    delay_timer: u8,
    /// Counts down at 60Hz, same as delay timer, but for sounds
    sound_timer: u8,
    /// 64 x 32 monochrome screen. Bool per pixel
    display: [bool; DISPLAY_WIDTH * DISPLAY_HEIGHT],
    /// Hex keypad
    keypad: [bool; 16],
}

#[allow(dead_code)] // fields written in new(); readers added as opcodes are implemented (Epic 2)
impl Chip8 {
    /// Initiates a new CHIP-8 emulator core
    /// # Examples
    /// ```
    /// let chip8 = chip8_core::Chip8::new();
    /// ```
    pub fn new() -> Self {
        let mut memory = [0u8; 4096];

        // NOTE: This effectively replaces the for loop bellow:
        // for i in 0..FONTSET_SIZE {
        //     memory[FONTSET_START_ADDRESS + i] = FONTSET[i];
        // }
        memory[FONTSET_START_ADDRESS..(FONTSET_SIZE + FONTSET_START_ADDRESS)].copy_from_slice(&FONTSET);

        Self {
            memory,
            v: [0u8; 16],
            i: 0u16,
            pc: START_ADDRESS,
            stack: [0u16; 16],
            sp: 0u8,
            delay_timer: 0u8,
            sound_timer: 0u8,
            display: [false; DISPLAY_WIDTH * DISPLAY_HEIGHT],
            keypad: [false; 16],
        }
    }

    /// Loads a rom at the program starting address of the CHIP-8 emulator.
    /// # Errors
    ///
    /// Will return `RomTooLarge { size: rom_size }` if `rom` is larger than the allowed ROM size of 3584 bytes (`MAX_ROM_SIZE`).
    #[must_use = "Must check Result to ensure ROM is loaded"]
    pub fn load_rom(&mut self, rom: &[u8]) -> Result<()> {
        let rom_size = rom.len();

        if rom_size > MAX_ROM_SIZE {
            return Err(Error::RomTooLarge { size: rom_size });
        }

        let start_address = START_ADDRESS as usize;
        let end_address = rom_size + start_address;

        self.memory[start_address..end_address].copy_from_slice(rom);

        Ok(())
    }

    /// Process the program instruction, dispatches it, and moves to the next availabe opcode
    /// # Errors
    ///
    /// Will return `chip8_core::Error` if it is unable to decode and execute a rom instruction.
    fn tick(&mut self) -> Result<()> {
        let opcode_start = self.pc as usize;
        let opcode_end = opcode_start + 1;

        if self.pc >= 0xFFF {
            return Err(Error::InvalidAddress(self.pc));
        }

        let opcode = u16::from_be_bytes([self.memory[opcode_start], self.memory[opcode_end]]);

        self.increase_program_counter();

        // NOTE: The ? operator is equivalent to doing the below after called
        // self.decode_and_executed
        // if let Err(failed_decode) = result {
        //     return Err(failed_decode);
        // }
        self.decode_and_execute(opcode)?;

        Ok(())
    }

    /// Moves emulator program counter to next instruction.
    pub fn increase_program_counter(&mut self) {
        self.pc += 2;
    }
}

impl Default for Chip8 {
    /// Initiates a new CHIP-8 emulator core with default configuration.
    /// # Examples
    /// ```
    /// let chip8 = chip8_core::Chip8::default();
    /// ```
    fn default() -> Self {
        Self::new()
    }
}

#[cfg(test)]
mod chip_core_tests {
    use pretty_assertions::assert_eq;

    use crate::{Chip8, Error, FONTSET, FONTSET_SIZE, FONTSET_START_ADDRESS, MAX_ROM_SIZE, START_ADDRESS};

    #[test]
    fn new() {
        let fontset_start_address = FONTSET_START_ADDRESS;
        let fontset_end_address = FONTSET_START_ADDRESS + FONTSET_SIZE;
        let chip8_emulator = Chip8::new();

        assert_eq!(chip8_emulator.pc, START_ADDRESS);
        assert_eq!(&chip8_emulator.memory[fontset_start_address..fontset_end_address], &FONTSET);
    }

    #[test]
    fn load_rom_succeeds() {
        let rom: [u8; MAX_ROM_SIZE] = rand::random();
        let mut emulator = Chip8::default();
        let start_address = START_ADDRESS as usize;
        let end_address = rom.len() + start_address;
        let result = emulator.load_rom(&rom);

        assert!(result.is_ok());
        assert_eq!(&emulator.memory[start_address..end_address], &rom);
    }

    #[test]
    fn load_rom_empty_rom_succeeds() {
        let empty_rom = [0u8; 0];
        let mut emulator = Chip8::default();
        let result = emulator.load_rom(&empty_rom);

        assert!(result.is_ok());
    }

    #[test]
    fn load_rom_rom_exceeds_max_size_fails() {
        let too_large_rom: [u8; MAX_ROM_SIZE + 1] = rand::random();
        let mut emulator = Chip8::default();
        let result = emulator.load_rom(&too_large_rom);

        assert!(result.is_err());
        assert_eq!(Error::RomTooLarge { size: MAX_ROM_SIZE + 1 }, result.unwrap_err());
    }

    #[test]
    fn tick_clear_opcode_succeeds() {
        let mut emulator = Chip8::default();
        let load_result = emulator.load_rom(&[0x00, 0xE0]);

        assert!(load_result.is_ok());

        let result = emulator.tick();

        assert!(result.is_ok());
        assert_eq!(emulator.pc, START_ADDRESS + 2);
    }

    #[test]
    fn tick_unknown_opcode_fails() {
        let mut emulator = Chip8::default();
        let load_result = emulator.load_rom(&[0x00, 0x12]);

        assert!(load_result.is_ok());

        let result = emulator.tick();

        assert!(result.is_err());
        assert_eq!(emulator.pc, START_ADDRESS + 2);
    }

    #[test]
    fn tick_opcode_skip_if_eq_immediate() {
        let mut emulator = Chip8::default();
        let load_result = emulator.load_rom(&[0x30, 0x01]);

        emulator.v[0] = 1;

        assert!(load_result.is_ok());

        let result = emulator.tick();

        assert!(result.is_ok());
        assert_eq!(emulator.pc, START_ADDRESS + 4);
    }

    #[test]
    fn tick_opcode_does_not_skip_if_immediate_ne_value() {
        let mut emulator = Chip8::default();
        let load_result = emulator.load_rom(&[0x35, 0x02]);

        emulator.v[5] = 1;

        assert!(load_result.is_ok());

        let result = emulator.tick();

        assert!(result.is_ok());
        assert_eq!(emulator.pc, START_ADDRESS + 2);
    }

    #[test]
    fn tick_opcode_skip_if_ne_immediate() {
        let mut emulator = Chip8::default();
        let load_result = emulator.load_rom(&[0x40, 0x01]);

        emulator.v[0] = 2;

        assert!(load_result.is_ok());

        let result = emulator.tick();

        assert!(result.is_ok());
        assert_eq!(emulator.pc, START_ADDRESS + 4);
    }

    #[test]
    fn tick_opcode_does_not_skip_if_immediate_eq_value() {
        let mut emulator = Chip8::default();
        let load_result = emulator.load_rom(&[0x45, 0x01]);

        emulator.v[5] = 1;

        assert!(load_result.is_ok());

        let result = emulator.tick();

        assert!(result.is_ok());
        assert_eq!(emulator.pc, START_ADDRESS + 2);
    }

    #[test]
    fn tick_opcode_skip_if_eq_register() {
        let mut emulator = Chip8::default();
        let load_result = emulator.load_rom(&[0x52, 0x30]);

        emulator.v[2] = 0x42;
        emulator.v[3] = 0x42;

        assert!(load_result.is_ok());

        let result = emulator.tick();

        assert!(result.is_ok());
        assert_eq!(emulator.pc, START_ADDRESS + 4);
    }

    #[test]
    fn tick_opcode_does_not_skip_if_register_ne_value() {
        let mut emulator = Chip8::default();
        let load_result = emulator.load_rom(&[0x52, 0x30]);

        emulator.v[2] = 0x42;
        emulator.v[3] = 0x43;

        assert!(load_result.is_ok());

        let result = emulator.tick();

        assert!(result.is_ok());
        assert_eq!(emulator.pc, START_ADDRESS + 2);
    }

    #[test]
    fn tick_opcode_skip_if_ne_register() {
        let mut emulator = Chip8::default();
        let load_result = emulator.load_rom(&[0x92, 0x30]);

        emulator.v[2] = 0x42;
        emulator.v[3] = 0x43;

        assert!(load_result.is_ok());

        let result = emulator.tick();

        assert!(result.is_ok());
        assert_eq!(emulator.pc, START_ADDRESS + 4);
    }

    #[test]
    fn tick_opcode_does_not_skip_if_register_eq_value() {
        let mut emulator = Chip8::default();
        let load_result = emulator.load_rom(&[0x92, 0x30]);

        emulator.v[2] = 0x42;
        emulator.v[3] = 0x42;

        assert!(load_result.is_ok());

        let result = emulator.tick();

        assert!(result.is_ok());
        assert_eq!(emulator.pc, START_ADDRESS + 2);
    }

    #[test]
    fn tick_opcode_invalid_5xy_last_nibble_fails() {
        let mut emulator = Chip8::default();
        let load_result = emulator.load_rom(&[0x52, 0x31]);

        assert!(load_result.is_ok());

        let result = emulator.tick();

        assert!(result.is_err());
        assert_eq!(Error::InvalidOpcode(0x5231), result.unwrap_err());
    }

    #[test]
    fn tick_opcode_invalid_9xy_last_nibble_fails() {
        let mut emulator = Chip8::default();
        let load_result = emulator.load_rom(&[0x92, 0x31]);

        assert!(load_result.is_ok());

        let result = emulator.tick();

        assert!(result.is_err());
        assert_eq!(Error::InvalidOpcode(0x9231), result.unwrap_err());
    }
}
