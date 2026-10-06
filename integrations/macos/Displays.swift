// Discover active physical displays without invoking OBS source-property lists.
import AppKit
import CoreGraphics

var rows: [String] = []
for screen in NSScreen.screens {
    guard let number = screen.deviceDescription[NSDeviceDescriptionKey("NSScreenNumber")] as? NSNumber else { continue }
    let display = CGDirectDisplayID(number.uint32Value)
    guard let reference = CGDisplayCreateUUIDFromDisplayID(display),
          let uuid = CFUUIDCreateString(nil, reference.takeRetainedValue()) else { continue }
    let mode = CGDisplayCopyDisplayMode(display)
    let width = mode?.pixelWidth ?? CGDisplayPixelsWide(display)
    let height = mode?.pixelHeight ?? CGDisplayPixelsHigh(display)
    let name = screen.localizedName.replacingOccurrences(of: "\t", with: " ").replacingOccurrences(of: "\n", with: " ")
    rows.append("\(uuid)\t\(name) (\(width) × \(height))")
}
print(rows.joined(separator: "\n"))
