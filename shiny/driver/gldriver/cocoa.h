#include <stdint.h>

void startDriver(void);
void stopDriver(void);
void makeCurrentContext(uintptr_t ctx);
uintptr_t newWindow(int width, int height);
uintptr_t showWindow(uintptr_t id);
uint64_t threadID();
