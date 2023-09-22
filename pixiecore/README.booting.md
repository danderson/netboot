# How it works

Pixiecore implements four different, but related protocols in one
binary, which together can take a network card to boot system into
Linux. They are: ProxyDHCP, PXE, TFTP, and HTTP. Let's
walk through the network boot process.

![Boot process](https://cdn.rawgit.com/google/netboot/master/pixiecore/bootgraph.svg)

## Step 1: DHCP/ProxyDHCP

Network boot is initiated by PXE ROM firmware that is embedded in
a capable network card.

The first thing a PXE ROM does is request a configuration through
DHCP request with additional PXE options set to indicate that it wants
to netboot (on a lower level it is `DHCPDISCOVER` with
[Option 60](https://en.wikipedia.org/wiki/Dynamic_Host_Configuration_Protocol#DHCP_client_vendor_identification)).
It expects a reply that mirrors some of these options, and
includes boot instructions in addition to network configuration.

The normal way of providing these options is to edit your DHCP
server's configuration to provide them to clients that identify
themselves as PXE clients. Unfortunately, reconfiguring your network's
DHCP server is tedious at best, and impossible if you DHCP server is
built into a consumer router, or managed by someone else.

Pixiecore instead uses a feature of the PXE specification called
_ProxyDHCP_. As you might guess from the name, ProxyDHCP is not a
proxy at all (yeah, the PXE spec is like that), but a second DHCP
server that only provides PXE configuration.

When the PXE ROM sends out a `DHCPDISCOVER`, it gets two replies back:
one containing only network configuration from the primary DHCP server
(no PXE options), and one containing only PXE DHCP options from the
ProxyDHCP server. The PXE firmware combines the two, and continues as
if the primary server had provided all of the configuration.

The client will finish network configuration with the primary DHCP
server (we're not involved with that), and will then proceed with the
next steps of booting.

## Step 1.5: PXE-ish

For classic BIOS clients, the ProxyDHCP response points to a TFTP
server and filename, and we go straight to step 2. For UEFI firmwares,
however, there's an additional step.

Sadly, many UEFI firmwares in the wild don't implement PXE properly,
and fail to chainload correctly if you send then a ProxyDHCP response
pointing directly to a TFTP server.

To get UEFI clients to boot reliably, we need to send them a ProxyDHCP
response that is invalid according to the PXE
specification. Specifically, a reply that lacks DHCP option 43 (PXE
Vendor Options).

Once the UEFI client has configured its network, it will then send a
DHCPREQUEST packet to port 4011 of our ProxyDHCP server. This is the
"PXE Boot Server" port, another relatively obscure part of the PXE
specification that allows PXE firmwares to display boot menus
natively, among other things.

Like our ProxyDHCP response, the PXE boot request and response in this
exchange are not valid according to the PXE specification, since they
both lack DHCP option 43 but include other PXE-specific options. Our
response to this request is essentially what we told BIOS clients in
step 1: here's a TFTP server and filename, go boot that.

So, UEFI clients need to do this little indirection before catching up
with its BIOS cousin.

### What is this strange protocol?

I haven't fully verified this yet, but the protocol seems to be
"BINL", a Microsoft proprietary fork of PXE that was introduced in the
early days of EFI.

There's no public specification for this protocol, but there is an
open-source implementation of a BINL client in the form of the
TianoCore EDK2 UEFI firmware. We can also examine packet captures of
machines being booted by "Windows Deployment Services", the service
that performs network installation of windows, and see that they use
this protoocl.

Both of these secondary sources strongly indicate that what we're
actually doing here is telling the UEFI client to use BINL in our
ProxyDHCP response, and then telling it to use TFTP in our BINL
response.

Modern UEFI firmwares (e.g. OVMF, derived from the TianoCore codebase)
support both standard PXE and this BINL variant, if BINL is what this
is. However, many firmwares that are still shipping in new devices
seem to only support BINL, which makes BINL the lowest common
denominator that has the best chance of booting all UEFI clients.

This is a somewhat sad state of affairs given that Intel provides an
open-source reference UEFI implementation that has supported PXE for a
long time. However, industry practice seems to be to maintain
seldom-to-never updated private forks of TianoCore, with extensive
non-public modifications. As a result, it's likely we'll be stuck in
this situation for a long time to come.

## Step 2: TFTP

TFTP is, as the name suggests, a trivial protocol for transferring
files. I have found some PXE ROMs that manage to add unnecessary
complexity even to that, but by and large, this step is
straightforward.

However, TFTP is quite slow, because it doesn't support transfer
windows (well, it does, but it's an extension defined in an RFC
published in 2015, so guess how many PXE ROMs implement it...). As a
result, you must pay one round-trip per ~1500 bytes transferred, and
even on a gigabit network, that slows things down.

Given that some netboot images are quite large (CoreOS clocks in at
almost 200MB), what we really want is to switch to a more efficient
protocol. That's where iPXE comes in.

iPXE is a small bootloader that knows how to boot Linux kernels, and
can speak HTTP. iPXE is between 50kB and 900kB (depending on the
architecture and BIOS vs. UEFI), which even over TFTP is very fast to
transfer.

Thus, Pixiecore uses TFTP only to transfer iPXE, and from there steers
to HTTP for the rest of the loading process.

## Step 3: ProxyDHCP, again

Unlike some other bootloaders like PXELINUX, iPXE does not reuse the
firmware's preexisting network settings. Instead, it starts the
process all over again with a DHCP request. Again, we send it a
ProxyDHCP response.

To break the infinite loop here, we can detect in the DHCP request
that the client is iPXE, and so we serve up a different response, one
that just points to an HTTP URL as the boot filename. iPXE interprets
this as a script (a sequence of iPXE commands, with minimal control
flow) that it should download and run.

One more catch is that iPXE has a race condition: when configuring
DHCP, if it receives the regular DHCP response before the ProxyDHCP
response, it will quickly finish configuring the network... and then
complain that it has no boot instructions. To counteract this, we
embed an iPXE script in the iPXE binary itself, telling it to retry
network configuration until it gets a boot filename out of it. So,
we're actually chainloading from one iPXE script (embedded) to another
(from HTTP).

## HTTP

We've finally crawled our way up to the late nineties - we can speak
HTTP! Pixiecore's HTTP server is wonderfully familiar and normal. It
just serves up a trivial iPXE script telling it to boot a Linux
kernel, and the user-provided kernel and initrd files.

iPXE grabs all of that, and finally, Linux boots.

## Recap

This is what the whole boot process looks like on the wire.

### Dramatis Personae

- **PXE ROM**, a brittle firmware burned into the network card.
- **DHCP server**, a plain old DHCP server providing network configuration.
- **Pixiecore**, the Hero and server of ProxyDHCP, PXE, TFTP and HTTP.
- **iPXE**, an open source [bootloader](http://ipxe.org).

### Timeline

- PXE ROM starts, broadcasts `DHCPDISCOVER`.
- DHCP server responds with a `DHCPOFFER` containing network configs.
- Pixiecore's ProxyDHCP server responds with a `DHCPOFFER` listing a TFTP file (BIOS) or BINL options (UEFI).
- PXE ROM does a `DHCPREQUEST`/`DHCPACK` exchange with the DHCP server to get a network configuration.
- (UEFI only) PXE ROM sends a `DHCPREQUEST` to Pixiecore's "PXE" server, asking for boot instructions.
- (UEFI only) Pixiecore's "PXE" server responds with a `DHCPACK` listing a TFTP file.
- PXE ROM downloads iPXE from Pixiecore's TFTP server, and hands off to iPXE.
- iPXE starts, broadcasts `DHCPDISCOVER`.
- DHCP server responds with a `DHCPOFFER` containing network configs.
- Pixiecore's ProxyDHCP server responds with a `DHCPOFFER` listing an HTTP URL.
- iPXE does a `DHCPREQUEST`/`DHCPACK` exchange with the DHCP server to get a network configuration.
- iPXE fetches its boot script from Pixiecore's HTTP server.
- iPXE fetches a kernel and ramdisk from Pixiecore's HTTP server, and boots Linux.

# Known deviations from specifications

Pixiecore aims to be compliant with the relevant specifications for
TFTP, DHCP, and PXE. This section lists the places where Pixiecore
deliberately deviates from the spec to support buggy clients.

## Missing Client Machine Identifier (GUID) option

Some PXE ROMs don't send DHCP option 97, "Client Machine Identifier
(GUID)", in their DHCP and PXE requests. According to the PXE 2.1
specification and RFC 4578, this makes the requests non-compliant:

> This option MUST be present in all DHCP and PXE packets sent by PXE-compliant clients and servers.

Pixiecore's behavior implements "SHOULD" instead of "MUST": if a
client request has a GUID, Pixiecore's response will respond with a
GUID. If the client request has no GUID, Pixiecore omits option 97 in
its response.
