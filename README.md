# Metanoia EBM for MT-G5321

This repo contains a reverse-engineered implementation of a significant part of the Metanoia EBM protocol to control their MT-G5321 G.fast modem chip.

The reverse-engineering was carried out with an old Swisscom Internet Box Standard and a Swisscom DU-8000 SFP module, since there aren't many other such modules on the market it might have limited applicability to other modules.

It consists of a (sadly incomplete) spec in SPEC.md and two utilities, fwutil which can be used to extract and deobfuscate firmware from a Metanoia firmware container as well as ebmmanager which operates the module. Together they can be used to get these G.fast modems working on third-party hardware.

Sadly the firmware is not redistributable, thus you have to extract it from publicly-available firmware images.