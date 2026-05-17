use dns_lookup::lookup_addr;
use socket2::{Domain, Protocol, SockAddr, Socket, Type};
use std::{
    env, io,
    mem::MaybeUninit,
    net::{IpAddr, Ipv4Addr, SocketAddr, SocketAddrV4, ToSocketAddrs},
    process,
    time::{Duration, Instant},
};

const ECHO_REPLY: u8 = 0;
const ECHO_REQUEST: u8 = 8;
const TIME_EXCEEDED: u8 = 11;

fn main() -> io::Result<()> {
    let args: Vec<String> = env::args().collect();

    if args.len() < 2 {
        eprintln!("Usage: {} <host> [max_ttl] [probes] [timeout_ms]", args[0]);
        process::exit(1);
    }

    let host = &args[1];
    let max_ttl: u8 = args.get(2).and_then(|s| s.parse().ok()).unwrap_or(30);
    let probes: u16 = args.get(3).and_then(|s| s.parse().ok()).unwrap_or(3);
    let timeout = Duration::from_millis(args.get(4).and_then(|s| s.parse().ok()).unwrap_or(3000));

    let dst = resolve(host)?;
    let addr = SockAddr::from(SocketAddrV4::new(dst, 0));

    let socket = Socket::new(Domain::IPV4, Type::RAW, Some(Protocol::ICMPV4))?;
    let id = process::id() as u16;

    println!("Tracing route to {} [{}]\n", host, dst);

    for ttl in 1..=max_ttl {
        socket.set_ttl_v4(ttl as u32)?;
        print!("{:<3}", ttl);

        let mut hop_ip = None;
        let mut reached = false;
        let mut times = Vec::new();

        for probe in 0..probes {
            let seq = ((ttl as u16) << 8) | probe;
            let packet = packet(id, seq);
            let start = Instant::now();

            socket.send_to(&packet, &addr)?;

            match recv(&socket, id, seq, start, timeout)? {
                Some((ip, done, rtt)) => {
                    hop_ip.get_or_insert(ip);
                    reached |= done;
                    times.push(Some(rtt));
                }
                None => times.push(None),
            }
        }

        match hop_ip {
            Some(ip) => {
                let name = lookup_addr(&IpAddr::V4(ip)).unwrap_or_else(|_| "*".into());
                print!(" {:<35} {:<15}", name, ip);
            }
            None => print!(" {:<35} {:<15}", "*", "*"),
        }

        for time in times {
            match time {
                Some(t) => print!(" {:>8.3} ms", t.as_secs_f64() * 1000.0),
                None => print!(" {:>11}", "*"),
            }
        }

        println!();

        if reached {
            break;
        }
    }

    Ok(())
}

fn resolve(host: &str) -> io::Result<Ipv4Addr> {
    (host, 0)
        .to_socket_addrs()?
        .find_map(|a| match a {
            SocketAddr::V4(v4) => Some(*v4.ip()),
            _ => None,
        })
        .ok_or_else(|| io::Error::new(io::ErrorKind::NotFound, "IPv4 not found"))
}

fn packet(id: u16, seq: u16) -> Vec<u8> {
    let mut p = vec![0u8; 40];

    p[0] = ECHO_REQUEST;
    p[4..6].copy_from_slice(&id.to_be_bytes());
    p[6..8].copy_from_slice(&seq.to_be_bytes());

    for i in 8..p.len() {
        p[i] = i as u8;
    }

    let sum = checksum(&p);
    p[2..4].copy_from_slice(&sum.to_be_bytes());

    p
}

fn checksum(data: &[u8]) -> u16 {
    let mut sum = 0u32;

    for chunk in data.chunks(2) {
        let word = if chunk.len() == 2 {
            u16::from_be_bytes([chunk[0], chunk[1]]) as u32
        } else {
            (chunk[0] as u32) << 8
        };

        sum += word;
    }

    while sum >> 16 != 0 {
        sum = (sum & 0xffff) + (sum >> 16);
    }

    !(sum as u16)
}

fn recv(
    socket: &Socket,
    id: u16,
    seq: u16,
    start: Instant,
    timeout: Duration,
) -> io::Result<Option<(Ipv4Addr, bool, Duration)>> {
    socket.set_read_timeout(Some(timeout))?;

    let mut buf = [MaybeUninit::<u8>::uninit(); 4096];

    loop {
        let (size, from) = match socket.recv_from(&mut buf) {
            Ok(v) => v,
            Err(e) if e.kind() == io::ErrorKind::WouldBlock || e.kind() == io::ErrorKind::TimedOut => {
                return Ok(None);
            }
            Err(e) if e.kind() == io::ErrorKind::Interrupted => continue,
            Err(e) => return Err(e),
        };

        let data = unsafe {
            std::slice::from_raw_parts(buf.as_ptr() as *const u8, size)
        };

        if let Some(result) = parse(data, &from, id, seq, start.elapsed()) {
            return Ok(Some(result));
        }
    }
}

fn parse(
    data: &[u8],
    from: &SockAddr,
    id: u16,
    seq: u16,
    rtt: Duration,
) -> Option<(Ipv4Addr, bool, Duration)> {
    let fallback = from.as_socket_ipv4()?.ip().to_owned();

    if data.len() < 28 {
        return None;
    }

    let ip_len = ((data[0] & 0x0f) as usize) * 4;
    let src = if data[0] >> 4 == 4 {
        Ipv4Addr::new(data[12], data[13], data[14], data[15])
    } else {
        fallback
    };

    match data[ip_len] {
        ECHO_REPLY => {
            if echo_matches(data, ip_len, id, seq) {
                Some((src, true, rtt))
            } else {
                None
            }
        }
        TIME_EXCEEDED => {
            let inner_ip = ip_len + 8;
            let inner_len = ((data[inner_ip] & 0x0f) as usize) * 4;
            let inner_icmp = inner_ip + inner_len;

            if data.len() >= inner_icmp + 8
                && data[inner_icmp] == ECHO_REQUEST
                && echo_matches(data, inner_icmp, id, seq)
            {
                Some((src, false, rtt))
            } else {
                None
            }
        }
        _ => None,
    }
}

fn echo_matches(data: &[u8], offset: usize, id: u16, seq: u16) -> bool {
    data.len() >= offset + 8
        && u16::from_be_bytes([data[offset + 4], data[offset + 5]]) == id
        && u16::from_be_bytes([data[offset + 6], data[offset + 7]]) == seq
}