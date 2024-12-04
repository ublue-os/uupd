package drv

type SystemUpdater struct {
	Config          DriverConfiguration
	SystemDriver    SystemUpdateDriver
	Outdated        bool
	UpdateAvailable bool
}

func (up SystemUpdater) Steps() int {
	var steps = 0

	if up.UpdateAvailable && up.Config.Enabled {
		steps += 1
	}

	return steps
}

func (up SystemUpdater) New(dryrun bool) (SystemUpdater, error) {
	up.Config = DriverConfiguration{
		Title:       "System",
		Description: "System Update",
		Enabled:     true,
		DryRun:      dryrun,
	}

	if dryrun {
		up.Outdated = false
		return up, nil
	}

	systemDriver, err := GetSystemUpdateDriver()
	if err != nil {
		return up, err
	}
	up.SystemDriver = systemDriver

	outdated, err := up.SystemDriver.ImageOutdated()
	if err != nil {
		return up, err
	}

	up.Outdated = outdated
	return up, nil
}

func (up *SystemUpdater) Check() (bool, error) {
	if up.Config.DryRun || !up.Config.Enabled {
		return true, nil
	}
	updateAvailable, err := up.SystemDriver.UpdateAvailable()
	return updateAvailable, err
}

func (up SystemUpdater) Update() (*[]CommandOutput, error) {
	var final_output = []CommandOutput{}

	if up.Config.DryRun {
		return &final_output, nil
	}

	out, err := up.SystemDriver.Update()
	tmpout := CommandOutput{}.New(out, err)
	if err != nil {
		tmpout.SetFailureContext("System update")
	}
	final_output = append(final_output, *tmpout)

	return &final_output, nil
}
