# Glossary

The shared vocabulary for online-pong. Tickets, test names, identifiers, and docs use
these terms with these meanings. Add a term when a new concept appears; change one only
with the user's agreement.

| Term            | Meaning                                                                            |
|-----------------|------------------------------------------------------------------------------------|
| CHIP-8          | The 1970s virtual machine the emulator implements. SUPER-CHIP and XO-CHIP extend it |
| Mode            | Which machine variant the emulator runs: CHIP-8, SUPER-CHIP, or XO-CHIP             |
| Quirk           | A behaviour that differs between machine variants (shift, load and store, jump)     |
| ROM             | A program image loaded into memory at address `0x200`                               |
| Opcode          | One 16-bit instruction                                                              |
| Tick            | One fetch, decode, and execute cycle of a single opcode                             |
| Timer tick      | One 60 Hz decrement of the delay and sound timers                                   |
| Display buffer  | The monochrome pixel grid (64 by 32 in CHIP-8 mode)                                 |
| Sprite          | A run of bytes drawn onto the display buffer with XOR; overlap is a collision       |
| Keypad          | The 16-key hexadecimal input device                                                 |
| Snapshot        | A complete copy of emulator state that can be restored exactly                      |
| Rollback        | Restoring a snapshot and replaying inputs when a remote input arrives late          |
| Player          | A registered person with a name and match history                                   |
| Match           | One completed game between players, with scores and a winner                        |
| Session         | A live game in progress on the server                                               |
| Matchmaking     | Pairing queued players into a session                                               |
| Scoreboard      | Players ranked by match results for a game type                                     |
