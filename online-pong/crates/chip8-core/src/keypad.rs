//! Hex keypad state.
//!
//! The 16-key hex keypad is stored on [`crate::Chip8`] as `[bool; 16]`.
//! Key-press and key-release are fed in by the caller (the frontend); this
//! module houses keypad query helpers once opcodes `EX9E`, `EXA1`, and
//! `FX0A`.
