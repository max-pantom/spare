//go:build desktop

#import <Cocoa/Cocoa.h>
#include <stdlib.h>
#include <string.h>

extern void spareTrayAction(char *action);

@interface SpareTrayTarget : NSObject
@end

@implementation SpareTrayTarget
- (void)performAction:(NSMenuItem *)sender {
    spareTrayAction((char *)[[sender representedObject] UTF8String]);
}
@end

static NSStatusItem *SpareStatusItem = nil;
static SpareTrayTarget *SpareTrayTargetInstance = nil;
static BOOL SpareTrayVisible = YES;

static NSMenuItem *SpareActionItem(NSString *title, NSString *action) {
    NSMenuItem *item = [[NSMenuItem alloc] initWithTitle:title
                                                  action:@selector(performAction:)
                                           keyEquivalent:@""];
    [item setTarget:SpareTrayTargetInstance];
    [item setRepresentedObject:action];
    return [item autorelease];
}

void spare_tray_start(void) {
    dispatch_async(dispatch_get_main_queue(), ^{
        if (SpareStatusItem != nil) {
            return;
        }
        SpareTrayTargetInstance = [[SpareTrayTarget alloc] init];
        SpareStatusItem = [[[NSStatusBar systemStatusBar] statusItemWithLength:NSSquareStatusItemLength] retain];
        SpareStatusItem.button.title = @"S";
        SpareStatusItem.button.toolTip = @"Spare";
    });
}

void spare_tray_update(const char *statusValue, const char *openValue, const char *toggleValue, int hasInstance, int running) {
    char *statusCopy = strdup(statusValue ?: "No active job");
    char *openCopy = strdup(openValue ?: "Choose a job");
    char *toggleCopy = strdup(toggleValue ?: "");
    dispatch_async(dispatch_get_main_queue(), ^{
        NSString *status = [NSString stringWithUTF8String:statusCopy];
        NSString *openLabel = [NSString stringWithUTF8String:openCopy];
        NSString *toggleLabel = [NSString stringWithUTF8String:toggleCopy];
        free(statusCopy);
        free(openCopy);
        free(toggleCopy);
        if (SpareStatusItem == nil) {
            return;
        }
        NSMenu *menu = [[[NSMenu alloc] initWithTitle:@"Spare"] autorelease];
        NSMenuItem *heading = [[[NSMenuItem alloc] initWithTitle:@"Spare" action:nil keyEquivalent:@""] autorelease];
        [heading setEnabled:NO];
        [menu addItem:heading];
        NSMenuItem *state = [[[NSMenuItem alloc] initWithTitle:status action:nil keyEquivalent:@""] autorelease];
        [state setEnabled:NO];
        [menu addItem:state];
        [menu addItem:[NSMenuItem separatorItem]];
        [menu addItem:SpareActionItem(openLabel, hasInstance ? @"open_recipe" : @"choose")];
        if (hasInstance) {
            [menu addItem:SpareActionItem(@"Show QR", @"share")];
            [menu addItem:SpareActionItem(toggleLabel, @"toggle")];
            [menu addItem:SpareActionItem(@"Recent activity", @"activity")];
        }
        [menu addItem:SpareActionItem(@"Open Spare", @"open_spare")];
        [menu addItem:[NSMenuItem separatorItem]];
        [menu addItem:SpareActionItem(@"Quit Spare", @"quit")];
        SpareStatusItem.menu = menu;
        SpareStatusItem.visible = SpareTrayVisible;
    });
}

void spare_tray_set_visible(int visible) {
    dispatch_async(dispatch_get_main_queue(), ^{
        SpareTrayVisible = visible != 0;
        if (SpareStatusItem != nil) {
            SpareStatusItem.visible = SpareTrayVisible;
        }
    });
}

void spare_tray_stop(void) {
    dispatch_async(dispatch_get_main_queue(), ^{
        if (SpareStatusItem != nil) {
            [[NSStatusBar systemStatusBar] removeStatusItem:SpareStatusItem];
            [SpareStatusItem release];
            SpareStatusItem = nil;
        }
        [SpareTrayTargetInstance release];
        SpareTrayTargetInstance = nil;
    });
}
