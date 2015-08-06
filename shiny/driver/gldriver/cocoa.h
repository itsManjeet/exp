#include <stdint.h>

void startDriver();
void stopDriver();
void makeCurrentContext(uintptr_t ctx);
uintptr_t newWindow(int width, int height, uintptr_t* glctx);
void showWindow(uintptr_t id);
uint64_t threadID();
