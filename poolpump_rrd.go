package main

import "fmt"

func (r *Rrd) addTemp(name, title string, colorid, which int) {
	r.creator.DS(name, "GAUGE", "30", "-273", "1000")
	vname := fmt.Sprintf("t%d", which)
	cname := fmt.Sprintf("f%d", which)
	r.grapher.Def(vname, r.path, name, "AVERAGE")
	if name == "solar" {
		r.grapher.CDef(cname, vname+",10,/")
	} else {
		r.grapher.CDef(cname, "9,5,/,"+vname+",*,32,+")
	}
	r.grapher.Line(2.0, cname, colorStr(colorid), title)
}

func (ppc *PoolPumpController) createRrds() {
	ppc.tempRrd.addTemp("pump", "Pump", 8, 1)
	ppc.tempRrd.addTemp("weather", "Weather", 1, 2)
	ppc.tempRrd.addTemp("roof", "Roof", 2, 3)
	ppc.tempRrd.addTemp("solar", "SolRad w/sqm", 4, 4)
	ppc.tempRrd.addTemp("pool", "Pool", 0, 5)
	ppc.tempRrd.addTemp("target", "Target", 6, 6)
	ppc.tempRrd.AddStandardRRAs()
	ppc.tempRrd.Creator().Create(*ppc.config.forceRrd)

	tg := ppc.tempRrd.grapher
	tg.SetTitle("Temperatures and Solar Radiation")
	tg.SetVLabel("Degrees Farenheit")
	tg.SetRightAxis(1, 0.0)
	tg.SetRightAxisLabel("dekawatts/sqm")
	tg.SetSize(640, 300) // Config?
	tg.SetImageFormat("PNG")

	pc := ppc.pumpRrd.Creator()
	pc.DS("status", "GAUGE", "30", "-1", "10")
	pc.DS("solar", "GAUGE", "30", "-1", "10")
	pc.DS("manual", "GAUGE", "30", "-1", "10")
	ppc.pumpRrd.AddStandardRRAs()
	pc.Create(*ppc.config.forceRrd) // fails if already exists

	pg := ppc.pumpRrd.grapher
	pg.SetTitle("Pump Activity")
	pg.SetVLabel("Status Code")
	pg.SetUpperLimit(5.0)
	pg.SetRightAxis(1, 0.0)
	pg.SetRightAxisLabel("Status Code")
	pg.SetSize(640, 200) // Config?
	pg.SetImageFormat("PNG")

	pg.Def("t1", ppc.pumpRrd.path, "status", "AVERAGE")
	pg.Line(2.0, "t1", colorStr(0), "Pump Status")
	pg.Def("t2", ppc.pumpRrd.path, "solar", "AVERAGE")
	pg.Line(2.0, "t2", colorStr(2), "Solar Status")
	pg.Def("t3", ppc.pumpRrd.path, "manual", "AVERAGE")
	pg.Line(2.0, "t3", colorStr(6), "Manual Operation")
}

// Writes updates to RRD files and generates cached graphs
func (ppc *PoolPumpController) UpdateRrd() {
	update := fmt.Sprintf("N:%f:%f:%f:%f:%f:%f",
		ppc.pumpTemp.Temperature(), ppc.WeatherC(), ppc.roofTemp.Temperature(),
		ppc.weather.GetSolarRadiation(ppc.zipcode),
		ppc.runningTemp.Temperature(), *ppc.solar.target)
	Debug("Updating TempRrd: %s", update)
	err := ppc.tempRrd.Updater().Update(update)
	if err != nil {
		Error("Update failed for TempRrd {%s}: %s", update, err.Error())
	}

	solar := 0.01
	if ppc.switches.solar.isOn() {
		solar = 1.03
	}
	manual := 0.02
	if ppc.switches.ManualState() {
		manual = 1.06
	}
	update = fmt.Sprintf("N:%d.001:%0.3f:%0.3f", ppc.switches.State(), solar, manual)
	Debug("Updating PumpRrd: %s", update)
	err = ppc.pumpRrd.Updater().Update(update)
	if err != nil {
		Error("Could not create PumpRrd: %s", err.Error())
	}
}
