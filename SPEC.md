# Metanoia EBM specification (for MT-G5321)
*Note: This is all reverse-engineered and as such may be neither complete nor correct*

What Metanoia brands Ethernet Boot Management (EBM) consists of two different
protocols: a bootloader protocol used by the on-chip bootloader and an
operational protocol used by the firmware once it is loaded from an external
source.

If the modem does not have a sufficient amount of flash it will generally come
up in bootloader mode, waiting for the host to download firmware to it before
it can be used. If enough flash is present, the chip loads the firmware from
flash and comes up in operational mode.

## Common properties
All Metanoia EBM protocols are based on plain Ethernet II frames with protocol
number 0x6120. The MTU is 1500 bytes.

All integers use big-endian encoding, not just in the packets themselves but
also in the firmware.

## Bootloader protocol
The bootloader protocol consists of a simple header followed by the payload:

<table><!-- Markdown doesn't do joined cells, so this is HTML -->
<thead>
  <tr>
    <th>Byte 0 </th>
    <th>1</th>
    <th>2</th>
    <th>3</th>
  </tr>
</thead>
<tbody>
  <tr>
    <td colspan="2">Sequence Number</td>
    <td colspan="2">Payload Length</td>
  </tr>
  <tr>
    <td colspan="2">Payload Type</td>
    <td colspan="2">Payload ...</td>
  </tr>
</tbody>
</table>

The Sequence Number (16 bits) starts at 1 and is incremented for every new
packet sent by the host. The response contains the same sequence number as the
request it belongs to.

Payload Length contains the length of the payload in bytes, without counting
the length of the header itself. It it is limited to 1494 bytes.

Payload Type contains the type of payload in the packet. Known packet types are
documented in the following sections.

### AssociateRequest (Type 0x1)
The first request which needs to be sent to the bootloader. It gets sent to the
well-known EUI-48 (MAC) address 00-0e-ad-33-44-55 as the modem does not
have a valid address yet.

<table>
<thead>
  <tr>
    <th>Byte 0 </th>
    <th>1</th>
    <th>2</th>
    <th>3</th>
  </tr>
</thead>
<tbody>
  <tr>
    <td>0</td>
    <td>2</td>
    <td>3</td>
    <td>4</td>
  </tr>
  <tr>
    <td colspan="4">Modem assigned EUI-48</td>
  </tr>
  <tr>
    <td colspan="2"></td>
    <td>0</td>
    <td>0</td>
  </tr>
  <tr>
    <td>0</td>
    <td>1</td>
    <td>0</td>
    <td>0</td>
  </tr>
  <tr>
    <td>0</td>
    <td>2</td>
    <td>0</td>
    <td>0</td>
  </tr>
  <tr>
    <td>0</td>
    <td>3</td>
    <td>0</td>
    <td>0</td>
  </tr>
</tbody>
</table>

Most of the request are magic bytes which do not seem to be carrying
information as they just count up. The only important field is the EUI-48 (MAC)
address which contains the address which will be assigned to the modem.

The Response must be of type *AssociateResponse* (Type 0x02) of which only the
first byte is relevant as it contains the status code. Any value other than
zero indicates an error.

### Ack (Type 0x14)
This type is sent as a response to different commands which do not need to
return any data, but just a status code.

The first byte contains the status code, any value other than zero is
considered an error.

#### DownloadBegin (Type 0x11)
The request to be sent to the modem after a successful AssociateResponse.
This request (and all subsequent ones) now need to be sent to the EUI-48 (MAC)
address which was assigned in the Associate step.

| Byte 0  | 1   | 2   | 3   |
|---------|-----|-----|-----|
| 0xba    | 0   | 0   | 0   |
| 0       | 1   | 2   | 3   |
| 0xa     | 0xb | 0xc | 0xd |

This request seems to entirely consist of magic bytes. There does not seem to
be any parameters in the request.

The response must be of type *Ack*.

### DownloadRecord (Type 0x12)
For each record in the firmware (see firmware section) one of these requests
needs to be sent to the modem.

The payload is a single raw record.

The response must be of type *Ack*.

### DownloadEnd (Type 0x13)
The final request sent to the modem after all records were downloaded.

<table>
<thead>
  <tr>
    <th>Byte 0 </th>
    <th>1</th>
    <th>2</th>
    <th>3</th>
  </tr>
</thead>
<tbody>
  <tr>
    <td colspan="4">CRC-32 Checksum</td>
  </tr>
  <tr>
    <td>0xf4</td>
    <td>0xee</td>
    <td>0x00</td>
    <td>0xdd</td>
  </tr>
</tbody>
</table>

The request consists of a CRC-32 of the firmware followed by 4 magic bytes.
It is not currently known over which part of the firmware the CRC-32 is
calculated, but it is fairly certain that it is generated with an IEEE
polynomial.

The response must be of type *Ack*.

After the response has been sent by the modem, it automatically boots into the
just-downloaded firmware, exiting bootloader mode.

## Operational protocol
The operational protocol is very different from the bootloader protocol.
All packets contain the following header:

<table>
<thead>
  <tr>
    <th>Byte 0 </th>
    <th>1</th>
    <th>2</th>
    <th>3</th>
  </tr>
</thead>
<tbody>
  <tr>
    <td>Type</td>
    <td colspan="3">Sequence Number</td>
  </tr>
  <tr>
    <td></td>
    <td colspan="2">Payload Length</td>
    <td>Status Code</td>
  </tr>
</tbody>
</table>

Type contains the type of payload following the header.

Payload Length contains the length in bytes of the payload following this
header.

Status Code contains the status code associated with the packet.
Known status codes are:

| Type | Description                 |
|------|-----------------------------|
| 0    | OK                          |
| 1    | GTPI_NOT_FOUND              |
| 2    | INVALID_ACCESSING           |
| 3    | LENGTH_MISMATCH             |
| 4    | INVALID_VALUE               |
| 5    | PSD_ERROR                   |
| 6    | RMSC_ERROR                  |
| 7    | CONNECTED                   |
| 16   | LENGTH_EXCEEDS_PAYLOAD_SIZE |
| 17   | INCOMPLETE_CMD              |
| 18   | ACCESS_DENIED               |
| 177  | DISCONNECTED                |
| 224  | QUESTION                    |
| 225  | ANSWER_CORRECT              |
| 226  | ANSWER_WRONG                |
| 227  | OCCUPIED                    |
| 228  | FORCED_CONNECT              |
| 255  | DEFAULT_STATUS              |

In requests the status code is usually DEFAULT_STATUS (0xff).

The sequence number starts at zero and is incremented for every non-retransmitted packet.

### Connect (Type 0x31)
Communication with the modem uses a form of connection. A connection is initiated with the Connect call. Its payload consists of 2 4-byte unsigned integers, an `answer` value and a `flags` vlue.

The initial connect call to the mode is made with "answer" set to the all-ones value (0xffffffff) and flags set to 0x3c.

The modem will respond with status QUESTION and two 4-byte unsigned integers. The second of which is the `question`. Reference client code seems to hardcode the answers, so there is probably no way to automatically calculate the answer from the question.

If the answer is known, a second connect call is made with "answer" set to the answer and `flags` set to 0x0.

If the modem responds with status set to ANSWER_CORRECT, the connection is open and will stay open for as long as the client regularly sends commands to the modem (at least every 60s).

Known Question/Answer pairs:
| Question   | Answer     |
|------------|------------|
| 0x95743926 | 0x6e6f6961 |

### OIDs
The modem has OIDs which are similar to SNMP OIDs, but are formatted differently.

The on-wire encoding of OIDs is made up of 8 uint32s, arranged as follows:
| Position (uint32) | Description |
| 0 | Identifier 1 |
| 1 | Identifier 2 |
| 3 | Identifier 3 |
| 4 | Length (number of values, not bytes) |
| 5 | Offset (how many values to skip at the start) |
| 6 | Value Type |
| 7 | Unknown |

This is then followed by the raw data in case this request/respons carries a payload.

### Read MIB (Type 0x06)
This command reads the value of a OID. The payload is a OID as described above. If the OID is valid and exists, it returns OK status and the OID as a payload, followed by the contents of the OID.

### Write MIB (Type 0x07)
This command writes the value of a OID. The payload is a OID as described above followed by its value. It returns OK status as well as the just-written OID (without value appended) if the OID is valid
and the write succeeded.

### Events
These commands are sent by the modem and do not require a direct response, but may require alterning the behavior of the EBM client.

#### Logger Output (Type 0x61) 
These commands are sent regularly by the modem and contain structured log
entries. The log type is found as a 16-bit unsigned value from byte 20 to 22 of the payload. Known log types are:

| Value | Log Type |
| 1 | Modem Status |
| 4 | Error |

#### Console Output (Type 0x60)
These commands are sent regularly by the modem and contain unstructured logs from it. They just contain plain text.

#### Disconnect (Type 0x70)
This command is sent by the modem when it closed the connection to the client because of inactivity. The connection needs to be reestablished by performing the connection procedure again.

```
TODO: Document rest of commands as well as large enums
```

## Firmware
Metanoia firmware is distributed in their own firmware pack format which can contain multiple firmwares.

The type of firmware used by the MT-G5321 is 0x23210010.

The firmware data is obfuscated by XORing it with a repeating key, similar to
a Vigenère cipher. Simple statistical analysis and lots of zeros in the
plaintext allows for relatively easy key recovery. The key is relatively long
at 128 bytes and does not look like it was randomly generated.

As the firmware is obfuscated, the pack format contains the sizes of the
obfuscated records so that the loader knows the record boundaries as
each DownloadRecord request needs to contain exactly one obfuscated record.
If the deobfuscated records are available, these boundaries are inherent
in the encoding of the records.

The firmware itself consists of a set of records specifying an address and
chunk of binary data to put there, similar to how Motorola S-Records or Intel
HEX files work, but encoded in binary form to make them more compact.

The record format only consists of a single type of record. Each record starts
with a 4-byte integer containing the start address to put the data, followed by
another 4-byte integer containing the length of the data divided by 4 (so an
8-byte record would have a length of 2). This is then followed by the raw data.

The DSP core on the MT-G5321 is most likely a Tensilica (owned by Cadence)
Xtensa LX9 configured as Big-Endian.