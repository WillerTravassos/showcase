//! Display buffer constants and pixel helpers.
//!
//! The 64×32 monochrome framebuffer lives on [`crate::Chip8`] as a flat
//! `[bool; DISPLAY_WIDTH * DISPLAY_HEIGHT]` array. This module houses
//! drawing helpers for opcode `DXYN`.
