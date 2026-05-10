//! CHIP-8 assembler: `.s8` source text → `.ch8` bytecode.
//!
//! Accepts assembly source in a custom syntax and emits a raw byte sequence
//! suitable for loading into the CHIP-8 emulator via `load_rom`. No runtime
//! dependency on `chip8-core`; bytecode is the contract between the two crates.

#[cfg(test)]
mod tests {}
