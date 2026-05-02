use if_addrs::{get_if_addrs, IfAddr};

fn main() {
    let ifaces = match get_if_addrs() {
        Ok(val) => val,
        Err(err) => {
            eprintln!("Failed to get interfaces: {}", err);
            return;
        },
    };

    for iface in ifaces {
        if iface.is_loopback() {
            continue;
        }
        if let IfAddr::V4(addr) = iface.addr {
            println!("Iface: {}, address: {}, mask: {}", iface.name, addr.ip, addr.netmask);
            return;
        }
    }
}
