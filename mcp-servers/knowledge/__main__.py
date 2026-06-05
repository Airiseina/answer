import argparse

import uvicorn

from knowledge import create_app, mcp

if __name__ == "__main__":
    parser = argparse.ArgumentParser()
    parser.add_argument("--mode", choices=["stdio", "sse"], default="sse")
    parser.add_argument("--host", default="0.0.0.0")
    parser.add_argument("--port", type=int, default=8001)
    args = parser.parse_args()

    if args.mode == "stdio":
        mcp.run()
    else:
        app = create_app()
        uvicorn.run(app, host=args.host, port=args.port)
