//go:build darwin && cgo
#import <Foundation/Foundation.h>
#include <sys/stat.h>
#include <string.h>
char *omd_trash(const char *path, unsigned long long dev, unsigned long long ino) {
 @autoreleasepool {
  struct stat st;
  if (lstat(path, &st) || (unsigned long long)st.st_dev != dev || st.st_ino != ino || S_ISLNK(st.st_mode)) return strdup("Identity changed before Trash operation");
  NSError *error = nil;
  BOOL ok = [[NSFileManager defaultManager] trashItemAtURL:[NSURL fileURLWithPath:[NSString stringWithUTF8String:path]] resultingItemURL:nil error:&error];
  return ok ? NULL : strdup([[error localizedDescription] UTF8String]);
 }
}
