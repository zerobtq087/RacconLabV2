package labs

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"regexp"
	"sync"
)

// Topologías disponibles (archivos en gns3/volumes/ansible/topologies)
var archivosTopologia = map[string]bool{
	"basic_lan.json":     true,
	"star_topology.json": true,
	"vlan_topology.json": true,
}

var reNombreLab = regexp.MustCompile(`^[A-Za-z0-9_-]{3,40}$`)

var (
	ErrDespliegueEnCurso = errors.New("ya hay un despliegue en curso en este laboratorio; espera a que termine")
	ErrNoEsDueno         = errors.New("solo el dueño del laboratorio puede desplegar plantillas en él")
)

type VLAN struct {
	ID      int    `json:"id"`
	Subnet  string `json:"subnet"`
	Gateway string `json:"gateway"`
}

// Despliegue: lo que el maestro llena en la pantalla de plantillas
type Despliegue struct {
	NombreLab       string `json:"nombre_lab"`
	ArchivoJSON     string `json:"archivo_json"`
	Subnet          string `json:"subnet"`
	SerialSubnet    string `json:"serial_subnet"`
	RoutingProtocol string `json:"routing_protocol"`
	OSPFArea        int    `json:"ospf_area"`
	VLANs           []VLAN `json:"vlans"`
}

func esRedIPv4(cidr string) bool {
	ip, _, err := net.ParseCIDR(cidr)
	return err == nil && ip.To4() != nil
}

// Validar revisa TODO antes de tocar el contenedor (no se confía en el navegador)
func (d *Despliegue) Validar() error {
	switch {
	case !archivosTopologia[d.ArchivoJSON]:
		return errors.New("plantilla desconocida")
	case !reNombreLab.MatchString(d.NombreLab):
		return errors.New("nombre del laboratorio inválido (3-40: letras, números, - y _)")
	case !esRedIPv4(d.Subnet):
		return errors.New("red LAN inválida (formato 192.168.1.0/24)")
	case d.SerialSubnet != "" && !esRedIPv4(d.SerialSubnet):
		return errors.New("red de enlaces inválida (formato 10.0.0.0/24)")
	case d.RoutingProtocol != "static" && d.RoutingProtocol != "rip" && d.RoutingProtocol != "ospf":
		return errors.New("enrutamiento inválido: static, rip u ospf")
	case d.OSPFArea < 0 || d.OSPFArea > 65535:
		return errors.New("área OSPF inválida")
	case len(d.VLANs) > 6:
		return errors.New("máximo 6 VLANs")
	}
	if d.ArchivoJSON != "basic_lan.json" && d.SerialSubnet == "" {
		return errors.New("falta la red para los enlaces entre routers")
	}
	vistos := map[int]bool{}
	for _, v := range d.VLANs {
		_, red, err := net.ParseCIDR(v.Subnet)
		gw := net.ParseIP(v.Gateway)
		switch {
		case v.ID < 2 || v.ID > 4094:
			return fmt.Errorf("VLAN %d: el ID debe estar entre 2 y 4094", v.ID)
		case vistos[v.ID]:
			return fmt.Errorf("VLAN %d repetida", v.ID)
		case err != nil || red.IP.To4() == nil:
			return fmt.Errorf("VLAN %d: subred inválida", v.ID)
		case gw == nil || !red.Contains(gw):
			return fmt.Errorf("VLAN %d: el gateway no pertenece a %s", v.ID, v.Subnet)
		}
		vistos[v.ID] = true
	}
	return nil
}

// candados por laboratorio: un despliegue a la vez en cada contenedor
var desplegando sync.Map

// DesplegarPlantilla inyecta los parámetros en el JSON de la topología y corre
// el playbook de Ansible DENTRO del contenedor GNS3 del dueño. Puede tardar varios minutos.
func (s *Servicio) DesplegarPlantilla(ctx context.Context, matricula string, d *Despliegue) (string, error) {
	if err := d.Validar(); err != nil {
		return "", err
	}
	w, soyDueno, err := s.Repo.DeUsuario(ctx, matricula)
	if err != nil {
		return "", err
	}
	if w == nil {
		return "", ErrSinLaboratorio
	}
	if !soyDueno {
		return "", ErrNoEsDueno
	}
	if _, ocupado := desplegando.LoadOrStore(w.ID, true); ocupado {
		return "", ErrDespliegueEnCurso
	}
	defer desplegando.Delete(w.ID)

	// 1) Leer la topología base desde el volumen de Ansible del contenedor
	base, codigo, err := s.Docker.Ejecutar(ctx, w.ID, []string{"cat", "/root/ansible/topologies/" + d.ArchivoJSON})
	if err != nil || codigo != 0 {
		return "", fmt.Errorf("no se pudo leer la topología %s", d.ArchivoJSON)
	}

	// 2) Inyectar los parámetros del maestro
	var topo map[string]any
	if err := json.Unmarshal([]byte(base), &topo); err != nil {
		return "", fmt.Errorf("la topología %s no es JSON válido: %w", d.ArchivoJSON, err)
	}
	vlans := d.VLANs
	if vlans == nil {
		vlans = []VLAN{}
	}
	topo["project_settings"] = map[string]any{
		"subnet":           d.Subnet,
		"serial_subnet":    d.SerialSubnet,
		"vlans":            vlans,
		"routing_protocol": d.RoutingProtocol,
		"ospf_area":        d.OSPFArea,
	}
	modificado, err := json.Marshal(topo)
	if err != nil {
		return "", err
	}

	// 3) Escribirla en /tmp del contenedor (en base64, pasada como argumento: sin problemas de comillas)
	destino := "/tmp/raccoon_topologia.json"
	b64 := base64.StdEncoding.EncodeToString(modificado)
	if _, codigo, err := s.Docker.Ejecutar(ctx, w.ID, []string{
		"sh", "-c", `printf %s "$1" | base64 -d > "$2"`, "sh", b64, destino,
	}); err != nil || codigo != 0 {
		return "", errors.New("no se pudo escribir la topología en el laboratorio")
	}

	// 4) Ejecutar el playbook
	log.Printf("[PLANTILLAS] %s despliega %s (%s) en %s", matricula, d.NombreLab, d.ArchivoJSON, w.ID)
	salida, codigo, err := s.Docker.Ejecutar(ctx, w.ID, []string{
		"ansible-playbook", "/root/ansible/topologies/deploy_topology.yml",
		"-e", "project_name=" + d.NombreLab,
		"-e", "topology_file=" + destino,
		"-e", fmt.Sprintf("gns3_url=http://127.0.0.1:%d", w.PuertoBase),
		"-e", "gns3_token=" + w.Token,
		"-e", "gns3_host=127.0.0.1",
		"-e", "target_host=127.0.0.1",
	})
	if err != nil || codigo != 0 {
		log.Printf("[PLANTILLAS] %s falló (código %d): %s", w.ID, codigo, ultimas(salida, 2000))
		return "", fmt.Errorf("Ansible falló (código %d). Últimas líneas: %s", codigo, ultimas(salida, 400))
	}
	log.Printf("[PLANTILLAS] %s: %s desplegado", w.ID, d.NombreLab)
	return fmt.Sprintf("Topología %s desplegada en GNS3", d.NombreLab), nil
}
