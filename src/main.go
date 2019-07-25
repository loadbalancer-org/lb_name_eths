package main

import (
    "fmt"
    "github.com/dselans/dmidecode"
    "io/ioutil"
    "os"
    "path/filepath"
    "strings"
)

const sysPath string = "/sys/class/net/"
var onBoardPCIIDs []string

type byPCIID []netIface

type netIface struct {
    macAddress string
    onBoard bool
    currentIfaceAssignment string
    preferredIfaceAssignment string
    pciID string
    driver string
    //This is needed because mellanox cards
    devPort string
}

//Sort the ifaces by pciid
func (b byPCIID) sort() []netIface {
    var ifaces = b
    //split the ifaces slice into onboard and add in pci cards then sort them and return a single slice
    //with onboard interfaces first.
    var onBoard []netIface
    var PCICard []netIface

    for _,iface := range ifaces {
        if iface.onBoard {
            onBoard = append(onBoard, iface)
        } else {
            PCICard = append(PCICard, iface)
        }
    }
    return append(bubbleSortByPCIID(onBoard), bubbleSortByPCIID(PCICard)...)
}

//Bubble sort by pciid
func bubbleSortByPCIID(ifaces []netIface) []netIface {
    //bubble sort?
    length := len(ifaces)
    for {
        swapped := false
        for i := 0; i < length-1; i++ {
            //No ones really sure how the below if statement works....but it does....
            if ifaces[i].pciID > ifaces[i+1].pciID {
                ifaces[i], ifaces[i+1] = ifaces[i+1], ifaces[i]
                swapped = true
                //because mellanox has the same pciid for both interfaces. Could have done it a one line if statement but this is a bit clearer.
           } else if ifaces[i].pciID == ifaces[i+1].pciID && ifaces[i].devPort > ifaces[i+1].devPort {
               ifaces[i], ifaces[i+1] = ifaces[i+1], ifaces[i]
                swapped = true
            }
        }

        if swapped == false {
            break
        }
    }
    return ifaces
}

//We require a couple of values that are actually directory names.
//so after listing the path split it by separator and grab the last one
func getValueFromFilePath(path string) string {
    s := strings.Split(path, "/")
    return s[len(s)-1]

}

//Does the pciID of the network interface match a known onboard one?
func IsOnboardPCIIDMatch(nicPCIID string) bool {
    for _, onBoardPCIID := range onBoardPCIIDs {
        if onBoardPCIID == nicPCIID {
            return true
        }
    }
    return false
}

//Get the onboard pciids from dmidecode
func getOnboardNICS() {
    dmi := dmidecode.New()

    if err := dmi.Run(); err != nil {
        fmt.Printf("Unable to get dmidecode information this needs to be run as root. Error: %v\n", err)
        panic(err)
    }

    // You can search by record name
    byNameData, _ := dmi.SearchByName("Onboard Device")
    for _, record := range byNameData {
        if record["Type"] == "Ethernet" {
            onBoardPCIIDs = append(onBoardPCIIDs, record["Bus Address"])
        }
    }
}

//Work out if we are a virtual interface such as a bond/loopback adapter or a bridge, we are not interested in those.
func isVirtualInterface(inface string) bool {
    //The device folders do not exist for virtual interfaces bonds/loopback-adapters/bridges so we can ignore them.
        if _,err := os.Stat(sysPath + inface + "/device"); os.IsNotExist(err) {
                return true
        }
        return false
}

func displayResults(interfaceStore []netIface) {
    for _, iFace := range interfaceStore {
        fmt.Printf("Current Iface Name: %s\n", iFace.currentIfaceAssignment)
        fmt.Printf("Driver: %s\n", iFace.driver)
        fmt.Printf("MacAddress: %s\n", iFace.macAddress)
        fmt.Printf("PCIID: %s\n", iFace.pciID)
        fmt.Printf("Onboard: %t\n", iFace.onBoard)
        fmt.Printf("Prefered Iface Name: %s\n", iFace.preferredIfaceAssignment)
        fmt.Printf("Device Port (because mellanox): %s\n", iFace.devPort)
        fmt.Println()
    }
}

//Re-label the interfaces so that our sorted slice starts at eth0
func reLabelInterfaces(interfaceStore []netIface) []netIface {
    const interfacePrefix = "eth"
    for i := 0; i < len(interfaceStore); i++ {
       interfaceStore[i].preferredIfaceAssignment = fmt.Sprintf("%s%d",interfacePrefix,i)
    }

    return interfaceStore
}

//The return of this mimics the output of redhat name eths. we could return our own structure but it involves changing
//more code So this will do.
func displayFormattedOutput(interfaceStore []netIface) {
    for _, iFace := range interfaceStore {
        fmt.Printf("%s %s # %s %s\n", iFace.preferredIfaceAssignment, strings.ToUpper(iFace.macAddress), iFace.currentIfaceAssignment, iFace.driver)
    }
}
/**
This exists because the redhat_name_eths perl script was no longer working with modern hardware. To be fair it had
a good run as it was written in 2001. It was struggling to correctly sort the interfaces as on newer dells it the bus
they were attached to was no longer 0 as a result anything could appear anywhere. So this script was created.
 */
func main() {
    getOnboardNICS()

    fileInfo, err := ioutil.ReadDir(sysPath)
    if err != nil {
        panic(err)
    }

    var interfaceStore = []netIface{}
    //Loop through the /sys/class/net directory which contains simlinks to each of the interfaces in proc.
    for _, file := range fileInfo {

       if isVirtualInterface(file.Name()) {
           continue
       }
       iface := netIface{}

       iface.currentIfaceAssignment = file.Name()
       //get the kernel module it is running with.
       kernMod, err := filepath.EvalSymlinks(sysPath + file.Name() + "/device/driver")

       if err != nil {
           panic(err)
       }
       //get the kernel module
       iface.driver = getValueFromFilePath(kernMod)

       //get the mac address
       macAddress, err := ioutil.ReadFile(sysPath + file.Name() + "/address")
       //the mac address string contains a newline we need to shift this.
       if err != nil {
           panic(err)
       }
       macString := strings.TrimSuffix(string(macAddress), "\n")
       iface.macAddress = macString
       //get the pciID
       pciID, err := filepath.EvalSymlinks(sysPath + file.Name() + "/device")

       if err != nil {
           panic(err)
       }

       iface.pciID = getValueFromFilePath(pciID)
        //We need the device port because mellanox only returns one pciid. 
       devicePort, err := ioutil.ReadFile(sysPath + file.Name() + "/dev_port")
       if err != nil {
           panic(err)
       }

       iface.devPort = strings.TrimSuffix(string(devicePort), "\n")

       iface.onBoard = IsOnboardPCIIDMatch(iface.pciID)

       interfaceStore = append(interfaceStore, iface)
    }

    //randomise the returned interfaces used for testing.
    //rand.Seed(time.Now().UnixNano())
    //rand.Shuffle(len(interfaceStore), func (i,j int) {
    //    interfaceStore[i], interfaceStore[j] = interfaceStore[j], interfaceStore[i]
    //})
    //REMOVE ABOVE CODE, just trying mixing up values to get out what is available
    sorted := byPCIID.sort(interfaceStore)
    relabeled := reLabelInterfaces(sorted)
    displayFormattedOutput(relabeled)
}