//go:build darwin

package notify

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework Foundation -framework UserNotifications

#include <stdlib.h>
#import <Foundation/Foundation.h>
#import <UserNotifications/UserNotifications.h>

// Delegate that always presents the notification even when app is in foreground.
@interface SwellUNDelegate : NSObject <UNUserNotificationCenterDelegate>
@end

@implementation SwellUNDelegate
- (void)userNotificationCenter:(UNUserNotificationCenter *)center
       willPresentNotification:(UNNotification *)notification
         withCompletionHandler:(void (^)(UNNotificationPresentationOptions))completionHandler {
    completionHandler(UNNotificationPresentationOptionBanner |
                      UNNotificationPresentationOptionSound);
}
@end

static SwellUNDelegate *_swellDelegate = nil;

// Request auth and set delegate. Must be called on the main thread (or any
// thread — UNUserNotificationCenter is thread-safe). Uses sync.Once in Go.
// NOTE: On macOS 12+, UNUserNotificationCenter requires a valid code signature
// (at minimum ad-hoc) for the authorization dialog to appear.
static void swellbox_request_notify_auth(void) {
    @autoreleasepool {
        if (!_swellDelegate) {
            _swellDelegate = [[SwellUNDelegate alloc] init];
            [UNUserNotificationCenter currentNotificationCenter].delegate = _swellDelegate;
        }
        UNAuthorizationOptions opts =
            UNAuthorizationOptionAlert | UNAuthorizationOptionSound;
        [[UNUserNotificationCenter currentNotificationCenter]
            requestAuthorizationWithOptions:opts
                          completionHandler:^(BOOL granted, NSError *err) {
            if (err) {
                NSLog(@"[SwellBox] notify auth error: %@", err.localizedDescription);
            }
        }];
    }
}

static void swellbox_notify(const char *title, const char *body) {
    @autoreleasepool {
        UNMutableNotificationContent *content =
            [[UNMutableNotificationContent alloc] init];
        content.title = title ? [NSString stringWithUTF8String:title] : @"";
        content.body  = body  ? [NSString stringWithUTF8String:body]  : @"";
        content.sound = [UNNotificationSound defaultSound];

        // Unique identifier so each notification is independent.
        NSString *identifier = [[NSUUID UUID] UUIDString];
        UNNotificationRequest *req =
            [UNNotificationRequest requestWithIdentifier:identifier
                                                 content:content
                                                 trigger:nil];   // nil = deliver immediately

        [[UNUserNotificationCenter currentNotificationCenter]
            addNotificationRequest:req
             withCompletionHandler:^(NSError *err) {
            if (err) {
                NSLog(@"[SwellBox] notify deliver error: %@", err.localizedDescription);
            }
        }];
    }
}
*/
import "C"

import (
	"log"
	"sync"
	"unsafe"
)

var authOnce sync.Once

func ensurePermission() {
	authOnce.Do(func() {
		C.swellbox_request_notify_auth()
	})
}

func show(title, message string, isError bool) {
	ensurePermission()
	ct := C.CString(title)
	cm := C.CString(message)
	defer C.free(unsafe.Pointer(ct))
	defer C.free(unsafe.Pointer(cm))
	C.swellbox_notify(ct, cm)
	if isError {
		log.Printf("[notify:error] %s: %s", title, message)
	}
}
