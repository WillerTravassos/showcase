//! Memory layout constants and ROM loading.
//!
//! CHIP-8 has 4KB of RAM (`0x000`–`0xFFF`). The first 512 bytes are reserved
//! for the interpreter (fontset lives at `0x050`). Programs load at `0x200`.
