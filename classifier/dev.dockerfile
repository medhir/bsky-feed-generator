FROM python:3.12

WORKDIR /app

RUN pip install watchdog

COPY requirements.txt .
RUN pip install -r requirements.txt

EXPOSE 12000

CMD watchmedo auto-restart --directory=/app --pattern=*.py --recursive -- python app.py