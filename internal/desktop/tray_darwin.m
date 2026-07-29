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

static void SpareApplyIconState(int iconState) {
    if (SpareStatusItem == nil) {
        return;
    }
    NSString *title = @"S";
    NSFontWeight weight = NSFontWeightRegular;
    if (iconState == 1) {
        weight = NSFontWeightSemibold;
    } else if (iconState == 2) {
        title = @"S•";
        weight = NSFontWeightMedium;
    } else if (iconState == 3) {
        title = @"S!";
        weight = NSFontWeightSemibold;
    }
    NSFont *font = [NSFont systemFontOfSize:12 weight:weight];
    NSDictionary *attributes = @{NSFontAttributeName: font};
    SpareStatusItem.button.attributedTitle =
        [[[NSAttributedString alloc] initWithString:title attributes:attributes] autorelease];
}

void spare_tray_start(void) {
    dispatch_async(dispatch_get_main_queue(), ^{
        if (SpareStatusItem != nil) {
            return;
        }
        SpareTrayTargetInstance = [[SpareTrayTarget alloc] init];
        SpareStatusItem = [[[NSStatusBar systemStatusBar] statusItemWithLength:NSSquareStatusItemLength] retain];
        SpareApplyIconState(0);
        SpareStatusItem.button.toolTip = @"Spare";
    });
}

void spare_tray_update(
    const char *headlineValue,
    const char *statusValue,
    const char *openValue,
    const char *toggleValue,
    int hasInstance,
    int isDrop,
    int hasAddress,
    int needsAttention,
    int canReconnect,
    int iconState
) {
    char *headlineCopy = strdup(headlineValue ?: "No job");
    char *statusCopy = strdup(statusValue ?: "No active job");
    char *openCopy = strdup(openValue ?: "Choose a job");
    char *toggleCopy = strdup(toggleValue ?: "");
    dispatch_async(dispatch_get_main_queue(), ^{
        NSString *headline = [NSString stringWithUTF8String:headlineCopy];
        NSString *status = [NSString stringWithUTF8String:statusCopy];
        NSString *openLabel = [NSString stringWithUTF8String:openCopy];
        NSString *toggleLabel = [NSString stringWithUTF8String:toggleCopy];
        free(headlineCopy);
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
        NSMenuItem *job = [[[NSMenuItem alloc] initWithTitle:headline action:nil keyEquivalent:@""] autorelease];
        [job setEnabled:NO];
        [menu addItem:job];
        if ([status length] > 0) {
            NSMenuItem *state = [[[NSMenuItem alloc] initWithTitle:status action:nil keyEquivalent:@""] autorelease];
            [state setEnabled:NO];
            [menu addItem:state];
        }
        [menu addItem:[NSMenuItem separatorItem]];
        if (needsAttention) {
            if (canReconnect) {
                [menu addItem:SpareActionItem(@"Reconnect", @"reconnect")];
            }
        } else if (hasInstance) {
            [menu addItem:SpareActionItem(@"Show QR", @"share")];
            [menu addItem:SpareActionItem(openLabel, isDrop ? @"open_files" : @"open_recipe")];
            if (hasAddress) {
                [menu addItem:SpareActionItem(@"Copy address", @"copy_address")];
            }
            [menu addItem:SpareActionItem(toggleLabel, @"toggle")];
        } else {
            [menu addItem:SpareActionItem(openLabel, @"choose")];
        }
        if (!needsAttention || canReconnect) {
            [menu addItem:[NSMenuItem separatorItem]];
        }
        [menu addItem:SpareActionItem(@"Open Spare", @"open_spare")];
        [menu addItem:SpareActionItem(@"Settings", @"settings")];
        [menu addItem:SpareActionItem(@"Quit", @"quit")];
        SpareStatusItem.menu = menu;
        SpareStatusItem.visible = SpareTrayVisible;
        SpareApplyIconState(iconState);
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
