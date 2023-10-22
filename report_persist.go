package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
)

// loadEquipmentReports will load all of the equipment reports that are saved
// to disk.
func (gcas *GCAServer) loadEquipmentReports() error {
	filepath := filepath.Join(gcas.baseDir, "equipment-reports.dat")
	rawData, err := ioutil.ReadFile(filepath)
	if err != nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("unable to open reports file: %v", err)
		}
		f, err := os.Create(filepath)
		if err != nil {
			return fmt.Errorf("unable to create reports file: %v", err)
		}
		f.Close()
		return nil
	}

	// Check that the data is a sensisble length.
	if len(rawData) % 80 != 0 {
		return fmt.Errorf("reports file has an unexpected size")
	}

	// Parse all of the reports and integrate them into the state.
	for i := 0; i < len(rawData)/80; i++ {
		report, err := gcas.parseReport(rawData[i*80:i*80+80])
		if err != nil {
			return fmt.Errorf("corrupt report: %v", err)
		}
		gcas.integrateReport(report)
	}

	return nil
}

// saveEquipmentReport will save an equipment report to disk, so that the
// report will still be available after a restart.
func (gcas *GCAServer) saveEquipmentReport(ea EquipmentReport) error {
	filepath := filepath.Join(gcas.baseDir, "equipment-reports.dat")
	file, err := os.OpenFile(filepath, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("unable to open file to save equipment report: %v", err)
	}
	defer file.Close()
	_, err = file.Write(ea.Serialize())
	if err != nil {
		return fmt.Errorf("unable to write to file to save equipment report: %v", err)
	}
	return nil
}
