//go:build darwin

package notify

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework Foundation -framework UserNotifications

#include <stdlib.h>
#import <Foundation/Foundation.h>

@interface SwellNotificationDelegate : NSObject <NSUserNotificationCenterDelegate>
@end

@implementation SwellNotificationDelegate
- (BOOL)userNotificationCenter:(NSUserNotificationCenter *)center shouldPresentNotification:(NSUserNotification *)notification {
    return YES;
}
@end

static SwellNotificationDelegate *swellDelegate = nil;

static void swellbox_deliver(NSString *title, NSString *body) {
#pragma clang diagnostic push
#pragma clang diagnostic ignored "-Wdeprecated-declarations"
	NSUserNotification *notif = [[NSUserNotification alloc] init];
	notif.title = title ?: @"";
	notif.informativeText = body ?: @"";
	[[NSUserNotificationCenter defaultUserNotificationCenter] deliverNotification:notif];
#pragma clang diagnostic pop
}

static void swellbox_notify(const char *title, const char *body) {
	@autoreleasepool {
		NSString *t = title ? [NSString stringWithUTF8String:title] : @"";
		NSString *b = body ? [NSString stringWithUTF8String:body] : @"";
		swellbox_deliver(t, b);
	}
}

static void swellbox_request_notify_auth(void) {
	@autoreleasepool {
#pragma clang diagnostic push
#pragma clang diagnostic ignored "-Wdeprecated-declarations"
		if (!swellDelegate) {
			swellDelegate = [[SwellNotificationDelegate alloc] init];
			[NSUserNotificationCenter defaultUserNotificationCenter].delegate = swellDelegate;
		}
#pragma clang diagnostic pop
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
