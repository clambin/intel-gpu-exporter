package testutil

const SinglePayload = `{
	"period": {
		"duration": 1048.677745,
		"unit": "ms"
	},
	"frequency": {
		"requested": 0.000000,
		"actual": 0.000000,
		"unit": "MHz"
	},
	"interrupts": {
		"count": 0.000000,
		"unit": "irq/s"
	},
	"rc6": {
		"value": 99.999597,
		"unit": "%"
	},
	"power": {
		"GPU": 1.000000,
		"Package": 4.000000,
		"unit": "W"
	},
	"imc-bandwidth": {
		"reads": 503.442586,
		"writes": 51.315726,
		"unit": "MiB/s"
	},
	"engines": {
		"Render/3D": {
			"busy": 1.000000,
			"sema": 0.000000,
			"wait": 0.000000,
			"unit": "%"
		},
		"Blitter": {
			"busy": 2.000000,
			"sema": 0.000000,
			"wait": 0.000000,
			"unit": "%"
		},
		"Video": {
			"busy": 3.000000,
			"sema": 0.000000,
			"wait": 0.000000,
			"unit": "%"
		},
		"VideoEnhance": {
			"busy": 4.000000,
			"sema": 0.000000,
			"wait": 0.000000,
			"unit": "%"
		}
	},
	"clients": {
		"4293539623": {
			"name": "foo",
			"pid": "1427673",
			"memory": {
				"system": {
					"total": "274145280",
					"shared": "0",
					"resident": "147709952",
					"purgeable": "675840",
					"active": "36433920"
				}
			},
			"engine-classes": {
				"Render/3D": {
					"busy": "0.000000",
					"unit": "%"
				},
				"Blitter": {
					"busy": "0.000000",
					"unit": "%"
				},
				"Video": {
					"busy": "0.000000",
					"unit": "%"
				},
				"VideoEnhance": {
					"busy": "0.000000",
					"unit": "%"
				}
			}
		}
	}
}`
