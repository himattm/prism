## 2024-05-13 - Static Map Allocation for High-Frequency Function
**Learning:** Returning map literals dynamically within high-frequency functions causes heavy CPU allocation overhead. Allocating a package-level global map and referencing it directly creates a huge performance improvement in tight loops.
**Action:** When a function returning a large literal collection is called per frame or heavily repeated (like `ColorMap()`), pre-allocate the map globally or at initialization time.
