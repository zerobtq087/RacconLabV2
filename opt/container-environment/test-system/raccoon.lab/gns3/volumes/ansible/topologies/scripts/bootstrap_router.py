import sys
import json
import time
import ipaddress
from netmiko import ConnectHandler

gns3_host = sys.argv[1]
console_port = int(sys.argv[2])
node_data = json.loads(sys.argv[3])
routing_protocol = sys.argv[4].lower() if len(sys.argv) > 4 else 'none'
ospf_area = sys.argv[5] if len(sys.argv) > 5 else '0'
base_subnet_str = sys.argv[6] if len(sys.argv) > 6 else '172.16.0.0/16'
full_topology_str = sys.argv[7] if len(sys.argv) > 7 else '{}'

hostname = node_data.get('name', 'Router')

def build_dynamic_subnet_map(base_cidr, full_topology_json):
    top_data = json.loads(full_topology_json) if isinstance(full_topology_json, str) else full_topology_json
    project_settings = top_data.get('project_settings', {})
    
    subnet_map = {}

    # 1. Mapear VLANs explícitas desde el JSON de Postman
    custom_vlans = project_settings.get('vlans', [])
    for vlan in custom_vlans:
        v_id = vlan.get('id')
        v_subnet = vlan.get('subnet')
        if v_id and v_subnet:
            tag = f"vlan_{v_id}"
            subnet_map[tag] = {
                "network": ipaddress.ip_network(v_subnet, strict=False),
                "gateway": vlan.get("gateway")
            }

    # 2. Manejo dinámico de enlaces seriales (Usa serial_subnet de Postman si existe)
    serial_cidr = project_settings.get('serial_subnet', '10.0.0.0/24')
    serial_net = ipaddress.ip_network(serial_cidr, strict=False)
    
    # 3. Mapear subredes asignadas a tags que no sean VLANs (LANs genéricas y enlaces seriales)
    lan_net = ipaddress.ip_network(base_cidr, strict=False)
    
    current_serial_addr = serial_net.network_address
    current_lan_addr = lan_net.network_address

    # Identificar tags restantes en interfaces
    for n in top_data.get('nodes', []):
        for iface in n.get('interfaces', []):
            tag = iface.get('subnet_tag')
            if tag and tag not in subnet_map:
                mask_bits = iface.get('mask_bits', 24)
                
                # Si el tag o la interfaz es Serial, asigna desde el bloque de seriales
                if 'serial' in tag.lower() or mask_bits == 30 or 'Serial' in iface.get('name', ''):
                    sub = ipaddress.ip_network(f"{current_serial_addr}/{mask_bits}", strict=False)
                    subnet_map[tag] = {"network": sub, "gateway": None}
                    current_serial_addr = sub.broadcast_address + 1
                else:
                    sub = ipaddress.ip_network(f"{current_lan_addr}/{mask_bits}", strict=False)
                    subnet_map[tag] = {"network": sub, "gateway": None}
                    current_lan_addr = sub.broadcast_address + 1

    return subnet_map

# Generar el mapa dinámico completo
subnet_map = build_dynamic_subnet_map(base_subnet_str, full_topology_str)

device = {
    'device_type': 'cisco_ios_telnet',
    'host': gns3_host,
    'port': console_port,
    'username': '',
    'password': '',
    'secret': '',
    'global_delay_factor': 2,
    'fast_cli': False,
}

print(f"Conectando a {hostname} ({gns3_host}:{console_port}) | Protocolo: {routing_protocol.upper()}...")

try:
    net_connect = ConnectHandler(**device)
    net_connect.write_channel("\r\n\r\n")
    time.sleep(1)

    config_commands = [
        f"hostname {hostname}",
        "banner motd $ RaccoonLAB - Emulador de Redes para Media Superior y Superior - UTTT$",
        "no ip domain lookup",
        "ip routing"
    ]

    configured_networks = []
    phy_interfaces_opened = set()

    # Configuración de interfaces
    for iface in node_data.get('interfaces', []):
        iface_name = iface['name']
        
        if "GigabitEthernet" in iface_name:
            iface_name = iface_name.replace("GigabitEthernet", "FastEthernet")
        elif iface_name.startswith("Gi"):
            iface_name = iface_name.replace("Gi", "FastEthernet")

        if "." in iface_name:
            parent_iface = iface_name.split(".")[0]
            if parent_iface not in phy_interfaces_opened:
                config_commands.extend([
                    f"interface {parent_iface}",
                    "no shutdown",
                    "exit"
                ])
                phy_interfaces_opened.add(parent_iface)

        subnet_tag = iface.get('subnet_tag')
        host_num = iface.get('host_number', 1)
        vlan_id = iface.get('vlan')

        if subnet_tag in subnet_map:
            sub_info = subnet_map[subnet_tag]
            network_obj = sub_info["network"]
            
            if sub_info["gateway"]:
                ip_addr = sub_info["gateway"]
            else:
                ip_addr = str(network_obj[host_num])
                
            netmask = str(network_obj.netmask)

            config_commands.append(f"interface {iface_name}")
            
            if vlan_id:
                config_commands.append(f"encapsulation dot1Q {vlan_id}")

            config_commands.extend([
                f"ip address {ip_addr} {netmask}",
                "no shutdown",
                "exit"
            ])

            configured_networks.append({
                'network': str(network_obj.network_address),
                'netmask': netmask,
                'wildcard': str(network_obj.hostmask),
                'tag': subnet_tag
            })
        else:
            config_commands.extend([
                f"interface {iface_name}",
                "no shutdown",
                "exit"
            ])

    # Configuración de enrutamiento
    if routing_protocol == 'ospf':
        config_commands.extend([
            "router ospf 1",
            "log-adjacency-changes"
        ])
        for net in configured_networks:
            config_commands.append(f"network {net['network']} {net['wildcard']} area {ospf_area}")
        config_commands.append("exit")

    elif routing_protocol in ['rip', 'ripv2']:
        config_commands.extend([
            "router rip",
            "version 2",
            "no auto-summary"
        ])
        for net in configured_networks:
            config_commands.append(f"network {net['network']}")
        config_commands.append("exit")

    elif routing_protocol in ['static', 'estatico', 'estatica']:
        top_data = json.loads(full_topology_str) if isinstance(full_topology_str, str) else full_topology_str
        all_nodes = top_data.get('nodes', [])
        all_links = top_data.get('links', [])
        
        # Mapear qué subredes existen en la topología global y a qué nodo pertenecen
        tag_owners = {}
        for n in all_nodes:
            n_name = n.get('name')
            for iface in n.get('interfaces', []):
                stag = iface.get('subnet_tag')
                if stag and stag in subnet_map:
                    if stag not in tag_owners or 'lan' in stag.lower() or 'vlan' in stag.lower():
                        tag_owners[stag] = {
                            "network": subnet_map[stag]["network"],
                            "node": n_name
                        }

        # Identificar interfaces seriales locales y sus IPs del vecino directo
        neighbor_next_hops = {}
        for iface in node_data.get('interfaces', []):
            stag = iface.get('subnet_tag')
            if stag and ('sub' in stag.lower() or 'link' in stag.lower() or 'serial' in stag.lower()):
                for link in all_links:
                    node_a = link.get('node_a')
                    node_b = link.get('node_b')
                    
                    if node_a == hostname or node_b == hostname:
                        other_node = node_b if node_a == hostname else node_a
                        other_node_obj = next((n for n in all_nodes if n.get('name') == other_node and n.get('type') == 'cisco'), None)
                        if other_node_obj:
                            other_iface = next((i for i in other_node_obj.get('interfaces', []) if i.get('subnet_tag') == stag), None)
                            if other_iface:
                                net_obj = subnet_map[stag]["network"]
                                host_n = other_iface.get('host_number', 2)
                                neighbor_next_hops[other_node] = str(net_obj[host_n])

        # Generar comandos 'ip route'
        # Routers de borde (1 solo enlace serial): Default Route hacia el único vecino
        if len(neighbor_next_hops) == 1:
            single_neighbor = list(neighbor_next_hops.keys())[0]
            next_hop_ip = neighbor_next_hops[single_neighbor]
            config_commands.append(f"ip route 0.0.0.0 0.0.0.0 {next_hop_ip}")

        # Routers intermedios/centrales (Múltiples enlaces seriales): Rutas estáticas específicas
        else:
            for stag, info in tag_owners.items():
                owner = info["node"]
                net_obj = info["network"]
                
                if owner == hostname:
                    continue
                
                if owner in neighbor_next_hops:
                    next_ip = neighbor_next_hops[owner]
                    config_commands.append(f"ip route {net_obj.network_address} {net_obj.netmask} {next_ip}")
                else:
                    for neighbor, next_ip in neighbor_next_hops.items():
                        config_commands.append(f"ip route {net_obj.network_address} {net_obj.netmask} {next_ip}")

    net_connect.send_config_set(config_commands)
    net_connect.send_command("write memory")
    net_connect.disconnect()

    print(f"--- Configuración completada exitosamente en {hostname} ---")

except Exception as e:
    print(f"Error al configurar {hostname}: {str(e)}")
    sys.exit(1)
