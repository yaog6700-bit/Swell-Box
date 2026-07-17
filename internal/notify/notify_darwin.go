//go:build darwin

package notify

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework Foundation -framework UserNotifications

#include <stdlib.h>
#import <Foundation/Foundation.h>
#import <UserNotifications/UserNotifications.h>

// Deliver a local notification from this process so macOS uses Swell-Box.app's
// CFBundleIcon (pickaxe). beeep/osascript always shows Script Editor instead.
static void swellbox_deliver(NSString *title, NSString *body) {
	UNUserNotificationCenter *center = [UNUserNotificationCenter currentNotificationCenter];
	UNMutableNotificationContent *content = [UNMutableNotificationContent new];
	content.title = title ?: @"";
	content.body = body ?: @"";
	NSString *ident = [[NSUUID UUID] UUIDString];
	UNNotificationRequest *req =
	    [UNNotificationRequest requestWithIdentifier:ident content:content trigger:nil];
	[center addNotificationRequest:req withCompletionHandler:^(__unused NSError *error){
	}];
}

static void swellbox_notify(const char *title, const char *body) {
	@autoreleasepool {
		NSString *t = title ? [NSString stringWithUTF8String:title] : @"";
		NSString *b = body ? [NSString stringWithUTF8String:body] : @"";
		UNUserNotificationCenter *center = [UNUserNotificationCenter currentNotificationCenter];
		[center getNotificationSettingsWithCompletionHandler:^(UNNotificationSettings *settings) {
			UNAuthorizationStatus st = settings.authorizationStatus;
			if (st == UNAuthorizationStatusAuthorized ||
			    st == UNAuthorizationStatusProvisional ||
			    st == UNAuthorizationStatusEphemeral) {
				swellbox_deliver(t, b);
				return;
			}
			if (st == UNAuthorizationStatusNotDetermined) {
				UNAuthorizationOptions opts =
				    UNAuthorizationOptionAlert | UNAuthorizationOptionSound | UNAuthorizationOptionBadge;
				[center requestAuthorizationWithOptions:opts
				                      completionHandler:^(BOOL granted, __unused NSError *error) {
					if (granted) {
						swellbox_deliver(t, b);
					}
				}];
			}
			// Denied / restricted: no toast (tray still works).
		}];
	}
}

static void swellbox_request_notify_auth(void) {
	@autoreleasepool {
		UNUserNotificationCenter *center = [UNUserNotificationCenter currentNotificationCenter];
		UNAuthorizationOptions opts =
		    UNAuthorizationOptionAlert | UNAuthorizationOptionSound | UNAuthorizationOptionBadge;
		[center requestAuthorizationWithOptions:opts
		                      completionHandler:^(__unused BOOL granted, __unused NSError *error){
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
