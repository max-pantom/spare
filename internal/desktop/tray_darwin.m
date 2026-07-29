//go:build desktop

#import <Cocoa/Cocoa.h>
#include <stdlib.h>
#include <string.h>

extern void spareTrayAction(char *action);

@class SpareTrayTarget;

static NSStatusItem *SpareStatusItem = nil;
static SpareTrayTarget *SpareTrayTargetInstance = nil;
static NSMenu *SparePendingMenu = nil;
static NSImage *SpareMarkImage = nil;
static NSImage *SpareWarningImage = nil;
static BOOL SpareTrayVisible = YES;
static BOOL SpareMenuOpen = NO;
static int SpareCurrentIconState = -1;

@interface SpareTrayTarget : NSObject <NSMenuDelegate>
@end

@implementation SpareTrayTarget
- (void)performAction:(NSMenuItem *)sender {
    id representedObject = [sender representedObject];
    if (![representedObject isKindOfClass:[NSString class]]) {
        return;
    }
    spareTrayAction((char *)[(NSString *)representedObject UTF8String]);
}

- (void)menuWillOpen:(NSMenu *)menu {
    SpareMenuOpen = YES;
}

- (void)menuDidClose:(NSMenu *)menu {
    SpareMenuOpen = NO;
    if (SparePendingMenu == nil || SpareStatusItem == nil) {
        return;
    }
    [SpareStatusItem setMenu:SparePendingMenu];
    [SparePendingMenu release];
    SparePendingMenu = nil;
}

@end

static NSMenuItem *SpareActionItem(NSString *title, NSString *action) {
    NSMenuItem *item = [[NSMenuItem alloc] initWithTitle:title
                                                  action:@selector(performAction:)
                                           keyEquivalent:@""];
    [item setTarget:SpareTrayTargetInstance];
    [item setRepresentedObject:action];
    return [item autorelease];
}

static NSImage *SpareCreateMarkImage(void) {
    // A compact vector version of the Spare mark. It is treated as a template
    // image so macOS supplies the correct menu-bar color in every appearance.
    NSString *svg =
        @"<svg xmlns='http://www.w3.org/2000/svg' width='18' height='14' "
         "viewBox='0 0 334 229'>"
         "<path fill='#000' d='M0 4a4 4 0 0 1 4-4h66a4 4 0 0 1 4 4v137H0V4Z'/>"
         "<path fill='#000' d='M130 71c0-39 32-71 71-71h62c39 0 71 32 71 71v70h-74V79h-56v62h-74V71Z'/>"
         "<path fill='#000' d='M0 156h204v14H0zM260 156h74v14h-74z'/>"
         "<path fill='#000' d='M8 186h188a8 8 0 0 1-8 14H16a8 8 0 0 1-8-14ZM260 186h74v14h-74z'/>"
         "<path fill='#000' d='M42 216h120a12 12 0 0 1-12 13H54a12 12 0 0 1-12-13ZM260 216h74v13h-74z'/>"
         "</svg>";
    NSData *data = [svg dataUsingEncoding:NSUTF8StringEncoding];
    NSImage *image = [[NSImage alloc] initWithData:data];
    [image setSize:NSMakeSize(18.0, 14.0)];
    [image setTemplate:YES];
    return image;
}

static NSImage *SpareCreateWarningImage(void) {
    NSImage *image = [[NSImage alloc] initWithSize:NSMakeSize(18.0, 16.0)];
    [image lockFocus];
    [SpareMarkImage drawInRect:NSMakeRect(0.0, 3.0, 15.0, 11.7)
                      fromRect:NSZeroRect
                     operation:NSCompositingOperationSourceOver
                      fraction:1.0];
    NSDictionary *attributes = @{
        NSFontAttributeName: [NSFont systemFontOfSize:9.0 weight:NSFontWeightBold],
        NSForegroundColorAttributeName: [NSColor blackColor],
    };
    [@"!" drawAtPoint:NSMakePoint(13.0, 3.5) withAttributes:attributes];
    [image unlockFocus];
    [image setTemplate:YES];
    return image;
}

static void SpareApplyIconState(int iconState) {
    if (SpareStatusItem == nil) {
        return;
    }

    if (SpareMarkImage == nil) {
        SpareMarkImage = SpareCreateMarkImage();
    }
    if (SpareWarningImage == nil) {
        SpareWarningImage = SpareCreateWarningImage();
    }

    if (SpareCurrentIconState == iconState &&
        [SpareStatusItem.button image] != nil) {
        return;
    }

    SpareCurrentIconState = iconState;
    [SpareStatusItem.button setTitle:@""];
    [SpareStatusItem.button setImage:iconState == 3 ? SpareWarningImage : SpareMarkImage];
    [SpareStatusItem.button setImagePosition:NSImageOnly];
    [SpareStatusItem.button setAlphaValue:1.0];
}

static NSMenu *SpareBuildMenu(
    NSString *headline,
    NSString *status,
    NSString *openLabel,
    NSString *toggleLabel,
    int hasInstance,
    int isDrop,
    int hasAddress,
    int needsAttention,
    int canReconnect
) {
    NSMenu *menu = [[NSMenu alloc] initWithTitle:@"Spare"];
    [menu setDelegate:SpareTrayTargetInstance];

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
        [menu addItem:SpareActionItem(@"View activity", @"activity")];
    } else {
        [menu addItem:SpareActionItem(openLabel, @"choose")];
    }

    [menu addItem:[NSMenuItem separatorItem]];
    [menu addItem:SpareActionItem(@"Open Spare", @"open_spare")];
    [menu addItem:SpareActionItem(@"Settings", @"settings")];
    [menu addItem:SpareActionItem(@"Quit", @"quit")];
    return menu;
}

void spare_tray_start(void) {
    dispatch_async(dispatch_get_main_queue(), ^{
        if (SpareStatusItem != nil) {
            return;
        }
        SpareTrayTargetInstance = [[SpareTrayTarget alloc] init];
        SpareStatusItem = [[[NSStatusBar systemStatusBar] statusItemWithLength:NSSquareStatusItemLength] retain];
        SpareCurrentIconState = -1;
        SpareApplyIconState(0);
        [SpareStatusItem.button setToolTip:@"Spare"];
        [SpareStatusItem.button setAccessibilityLabel:@"Spare"];
        [SpareStatusItem setVisible:SpareTrayVisible];
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

        NSMenu *menu = SpareBuildMenu(
            headline,
            status,
            openLabel,
            toggleLabel,
            hasInstance,
            isDrop,
            hasAddress,
            needsAttention,
            canReconnect
        );
        if (SpareMenuOpen) {
            [SparePendingMenu release];
            SparePendingMenu = menu;
        } else {
            [SpareStatusItem setMenu:menu];
            [menu release];
        }

        [SpareStatusItem setVisible:SpareTrayVisible];
        SpareApplyIconState(iconState);
    });
}

void spare_tray_set_visible(int visible) {
    dispatch_async(dispatch_get_main_queue(), ^{
        SpareTrayVisible = visible != 0;
        if (SpareStatusItem != nil) {
            [SpareStatusItem setVisible:SpareTrayVisible];
        }
    });
}

void spare_tray_stop(void) {
    dispatch_async(dispatch_get_main_queue(), ^{
        [SparePendingMenu release];
        SparePendingMenu = nil;
        SpareMenuOpen = NO;

        if (SpareStatusItem != nil) {
            [[SpareStatusItem menu] setDelegate:nil];
            [[NSStatusBar systemStatusBar] removeStatusItem:SpareStatusItem];
            [SpareStatusItem release];
            SpareStatusItem = nil;
        }
        [SpareTrayTargetInstance release];
        SpareTrayTargetInstance = nil;
        [SpareMarkImage release];
        SpareMarkImage = nil;
        [SpareWarningImage release];
        SpareWarningImage = nil;
        SpareCurrentIconState = -1;
    });
}
