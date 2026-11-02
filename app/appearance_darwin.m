#import <Cocoa/Cocoa.h>

// Wails installs the visual effect view; use the system sidebar material so it
// follows both light and dark appearances without painting over the webview.
void omdUseSidebarMaterial(void) {
    dispatch_async(dispatch_get_main_queue(), ^{
        for (NSWindow *window in [NSApp windows]) {
            for (NSView *view in [[window contentView] subviews]) {
                if ([view isKindOfClass:[NSVisualEffectView class]]) {
                    [(NSVisualEffectView *)view setMaterial:NSVisualEffectMaterialSidebar];
                }
            }
        }
    });
}
