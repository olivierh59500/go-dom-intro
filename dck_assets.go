package domintro

import "embed"

// DCKAssetAssets shares an embedded resource with the optional DCK version.
func DCKAssetAssets() embed.FS { return assets }

// DCKAssetYmData shares an embedded resource with the optional DCK version.
func DCKAssetYmData() []byte { return ymData }
